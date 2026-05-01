package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type Option func(*Service)

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		s.now = clock
	}
}

func WithCancelLeadTime(lead time.Duration) Option {
	return func(s *Service) {
		s.cancelLeadTime = lead
	}
}

type Service struct {
	repo           Repository
	now            func() time.Time
	cancelLeadTime time.Duration
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo:           repo,
		now:            time.Now,
		cancelLeadTime: time.Hour,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type SeatInput struct {
	Name   string
	Zone   string
	Type   string
	Active bool
}

type CreateBookingInput struct {
	SeatID      int64
	StartAt     time.Time
	EndAt       time.Time
	DisplayName string
}

type Report struct {
	From             time.Time     `json:"from"`
	To               time.Time     `json:"to"`
	TotalBookings    int           `json:"total_bookings"`
	CanceledBookings int           `json:"canceled_bookings"`
	BySeat           map[int64]int `json:"by_seat"`
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(strings.ToLower(in.Email))
	password := strings.TrimSpace(in.Password)
	if name == "" || email == "" || password == "" {
		return nil, domain.ErrInvalidInput
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
		CreatedAt:    s.now().UTC(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (string, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	password := strings.TrimSpace(in.Password)
	if email == "" || password == "" {
		return "", domain.ErrInvalidInput
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrInvalidCredentials
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrInvalidCredentials
	}

	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	session := &domain.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (*domain.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, domain.ErrUnauthorized
	}
	session, err := s.repo.GetSessionByToken(ctx, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	if s.now().UTC().After(session.ExpiresAt) {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	return user, nil
}

func (s *Service) ListAvailableSeats(ctx context.Context, startAt, endAt time.Time) ([]domain.Seat, error) {
	if err := validateInterval(startAt, endAt); err != nil {
		return nil, err
	}
	seats, err := s.repo.ListSeats(ctx)
	if err != nil {
		return nil, err
	}
	available := make([]domain.Seat, 0, len(seats))
	for _, seat := range seats {
		if !seat.Active {
			continue
		}
		conflict, err := s.repo.SeatHasConflict(ctx, seat.ID, startAt.UTC(), endAt.UTC())
		if err != nil {
			return nil, err
		}
		if !conflict {
			available = append(available, seat)
		}
	}
	return available, nil
}

func (s *Service) CreateBooking(ctx context.Context, userID int64, in CreateBookingInput) (*domain.Booking, error) {
	if userID <= 0 || in.SeatID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	if err := validateInterval(in.StartAt, in.EndAt); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if !in.StartAt.After(now) {
		return nil, domain.ErrInvalidInput
	}

	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	seat, err := s.repo.GetSeatByID(ctx, in.SeatID)
	if err != nil {
		return nil, err
	}
	if !seat.Active {
		return nil, domain.ErrSeatUnavailable
	}

	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings.BookingLimit <= 0 {
		return nil, domain.ErrInvalidInput
	}
	activeBookings, err := s.repo.CountActiveBookingsByUser(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if activeBookings >= settings.BookingLimit {
		return nil, domain.ErrLimitExceeded
	}

	conflict, err := s.repo.SeatHasConflict(ctx, in.SeatID, in.StartAt.UTC(), in.EndAt.UTC())
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, domain.ErrSeatUnavailable
	}

	displayName := strings.TrimSpace(in.DisplayName)

	booking := &domain.Booking{
		UserID:      userID,
		SeatID:      in.SeatID,
		StartAt:     in.StartAt.UTC(),
		EndAt:       in.EndAt.UTC(),
		Status:      domain.BookingStatusConfirmed,
		DisplayName: displayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateBooking(ctx, booking); err != nil {
		return nil, err
	}

	// For device-bound students we keep the user's name in sync with the most
	// recent display_name so that admin views and reports are readable even
	// without joining display_name.
	if displayName != "" {
		user, err := s.repo.GetUserByID(ctx, userID)
		if err == nil && user.DeviceID != "" && user.Name != displayName {
			_ = s.repo.UpdateUserName(ctx, userID, displayName)
		}
	}
	return booking, nil
}

// LoginAsDevice issues a session for a device-bound anonymous student.
// On the first call for a given deviceID the service creates a hidden user
// with role=user; subsequent calls return tokens for the same user. There is
// no password — the device id itself is the credential and is stored only on
// the device that generated it.
func (s *Service) LoginAsDevice(ctx context.Context, deviceID string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if len(deviceID) < 8 || len(deviceID) > 128 {
		return "", domain.ErrInvalidInput
	}

	user, err := s.repo.GetUserByDeviceID(ctx, deviceID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	if user == nil {
		now := s.now().UTC()
		newUser := &domain.User{
			Name:         "Студент",
			Email:        "device-" + deviceID + "@local.invalid",
			PasswordHash: "!device", // not used for password auth, never matches bcrypt
			Role:         domain.RoleUser,
			DeviceID:     deviceID,
			CreatedAt:    now,
		}
		if err := s.repo.CreateUser(ctx, newUser); err != nil {
			return "", err
		}
		user = newUser
	}

	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	session := &domain.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) CancelBooking(ctx context.Context, userID, bookingID int64) error {
	if userID <= 0 || bookingID <= 0 {
		return domain.ErrInvalidInput
	}
	booking, err := s.repo.GetBookingByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.UserID != userID {
		return domain.ErrForbidden
	}
	if booking.Status != domain.BookingStatusConfirmed {
		return domain.ErrConflictState
	}
	if booking.StartAt.Sub(s.now().UTC()) < s.cancelLeadTime {
		return domain.ErrBookingNotCancelable
	}
	return s.repo.UpdateBookingStatus(ctx, bookingID, domain.BookingStatusCanceled, s.now().UTC())
}

func (s *Service) ListUserBookings(ctx context.Context, userID int64) ([]domain.Booking, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	return s.repo.ListBookingsByUser(ctx, userID)
}

func (s *Service) CreateSeat(ctx context.Context, actorRole domain.Role, in SeatInput) (*domain.Seat, error) {
	if err := ensureAdmin(actorRole); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	zone := strings.TrimSpace(in.Zone)
	seatType := strings.TrimSpace(in.Type)
	if name == "" || zone == "" || seatType == "" {
		return nil, domain.ErrInvalidInput
	}

	now := s.now().UTC()
	seat := &domain.Seat{
		Name:      name,
		Zone:      zone,
		Type:      seatType,
		Active:    in.Active,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateSeat(ctx, seat); err != nil {
		return nil, err
	}
	return seat, nil
}

func (s *Service) UpdateSeat(ctx context.Context, actorRole domain.Role, seatID int64, in SeatInput) (*domain.Seat, error) {
	if err := ensureAdmin(actorRole); err != nil {
		return nil, err
	}
	if seatID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	seat, err := s.repo.GetSeatByID(ctx, seatID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(in.Name)
	zone := strings.TrimSpace(in.Zone)
	seatType := strings.TrimSpace(in.Type)
	if name == "" || zone == "" || seatType == "" {
		return nil, domain.ErrInvalidInput
	}

	seat.Name = name
	seat.Zone = zone
	seat.Type = seatType
	seat.Active = in.Active
	seat.UpdatedAt = s.now().UTC()

	if err := s.repo.UpdateSeat(ctx, seat); err != nil {
		return nil, err
	}
	return seat, nil
}

func (s *Service) DeleteSeat(ctx context.Context, actorRole domain.Role, seatID int64) error {
	if err := ensureAdmin(actorRole); err != nil {
		return err
	}
	if seatID <= 0 {
		return domain.ErrInvalidInput
	}
	hasFuture, err := s.repo.HasFutureBookingsForSeat(ctx, seatID, s.now().UTC())
	if err != nil {
		return err
	}
	if hasFuture {
		return domain.ErrConflictState
	}
	return s.repo.DeleteSeat(ctx, seatID)
}

func (s *Service) AdminUpdateBookingStatus(ctx context.Context, actorRole domain.Role, bookingID int64, status domain.BookingStatus) error {
	if err := ensureAdmin(actorRole); err != nil {
		return err
	}
	if bookingID <= 0 {
		return domain.ErrInvalidInput
	}
	if status != domain.BookingStatusCanceled && status != domain.BookingStatusCompleted && status != domain.BookingStatusConfirmed {
		return domain.ErrInvalidInput
	}

	booking, err := s.repo.GetBookingByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status == domain.BookingStatusCanceled || booking.Status == domain.BookingStatusCompleted {
		return domain.ErrConflictState
	}

	return s.repo.UpdateBookingStatus(ctx, bookingID, status, s.now().UTC())
}

func (s *Service) AdminListBookings(ctx context.Context, actorRole domain.Role) ([]domain.Booking, error) {
	if err := ensureAdmin(actorRole); err != nil {
		return nil, err
	}
	return s.repo.ListAllBookings(ctx)
}

func (s *Service) UpdateBookingLimit(ctx context.Context, actorRole domain.Role, limit int) error {
	if err := ensureAdmin(actorRole); err != nil {
		return err
	}
	if limit <= 0 || limit > 100 {
		return domain.ErrInvalidInput
	}
	return s.repo.SetBookingLimit(ctx, limit)
}

func (s *Service) BuildReport(ctx context.Context, actorRole domain.Role, from, to time.Time) (*Report, error) {
	if err := ensureAdmin(actorRole); err != nil {
		return nil, err
	}
	if !from.Before(to) {
		return nil, domain.ErrInvalidInput
	}

	bookings, err := s.repo.ListAllBookings(ctx)
	if err != nil {
		return nil, err
	}

	report := &Report{
		From:   from.UTC(),
		To:     to.UTC(),
		BySeat: map[int64]int{},
	}

	for _, booking := range bookings {
		if !overlapsWindow(booking.StartAt, booking.EndAt, from, to) {
			continue
		}
		report.TotalBookings++
		if booking.Status == domain.BookingStatusCanceled {
			report.CanceledBookings++
		}
		report.BySeat[booking.SeatID]++
	}

	return report, nil
}

func validateInterval(startAt, endAt time.Time) error {
	if startAt.IsZero() || endAt.IsZero() {
		return domain.ErrInvalidInput
	}
	if !startAt.Before(endAt) {
		return domain.ErrInvalidInput
	}
	return nil
}

func ensureAdmin(role domain.Role) error {
	if role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return nil
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func overlapsWindow(startAt, endAt, from, to time.Time) bool {
	return startAt.Before(to) && endAt.After(from)
}
