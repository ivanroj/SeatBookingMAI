package app

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

func TestRegisterFailsForDuplicateEmail(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	_, _ = svc.Register(context.Background(), RegisterInput{
		Name:     "User One",
		Email:    "user@mai.ru",
		Password: "secret123",
	})

	_, err := svc.Register(context.Background(), RegisterInput{
		Name:     "User Two",
		Email:    "user@mai.ru",
		Password: "secret123",
	})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestCreateBookingFailsOnTimeConflict(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))
	repo.settings.BookingLimit = 3

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")

	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	_, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat.ID,
		StartAt: now.Add(2*time.Hour + 15*time.Minute),
		EndAt:   now.Add(3*time.Hour + 15*time.Minute),
	})
	if !errors.Is(err, domain.ErrSeatUnavailable) {
		t.Fatalf("expected ErrSeatUnavailable, got %v", err)
	}
}

func TestCreateBookingFailsOnLimit(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))
	repo.settings.BookingLimit = 1

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat1 := repo.mustCreateSeat("A1")
	seat2 := repo.mustCreateSeat("A2")

	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat1.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	_, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat2.ID,
		StartAt: now.Add(4 * time.Hour),
		EndAt:   now.Add(5 * time.Hour),
	})
	if !errors.Is(err, domain.ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestCreateBookingSuccess(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))
	repo.settings.BookingLimit = 2

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")

	booking, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if booking.Status != domain.BookingStatusConfirmed {
		t.Fatalf("expected confirmed status, got %s", booking.Status)
	}
}

func TestCancelBookingForbiddenForAnotherUser(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	owner := repo.mustCreateUser("owner@mai.ru", domain.RoleUser)
	other := repo.mustCreateUser("other@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	booking := repo.mustCreateBooking(domain.Booking{
		UserID:  owner.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	err := svc.CancelBooking(context.Background(), other.ID, booking.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCancelBookingFailsWhenTooLate(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }), WithCancelLeadTime(time.Hour))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	booking := repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(30 * time.Minute),
		EndAt:   now.Add(90 * time.Minute),
		Status:  domain.BookingStatusConfirmed,
	})

	err := svc.CancelBooking(context.Background(), user.ID, booking.ID)
	if !errors.Is(err, domain.ErrBookingNotCancelable) {
		t.Fatalf("expected ErrBookingNotCancelable, got %v", err)
	}
}

func TestCancelBookingSuccess(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }), WithCancelLeadTime(time.Hour))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	booking := repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	err := svc.CancelBooking(context.Background(), user.ID, booking.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, err := repo.GetBookingByID(context.Background(), booking.ID)
	if err != nil {
		t.Fatalf("failed to get booking: %v", err)
	}
	if stored.Status != domain.BookingStatusCanceled {
		t.Fatalf("expected canceled status, got %s", stored.Status)
	}
}

func TestDeleteSeatFailsWhenFutureBookingExists(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(5 * time.Hour),
		EndAt:   now.Add(6 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	err := svc.DeleteSeat(context.Background(), domain.RoleAdmin, seat.ID)
	if !errors.Is(err, domain.ErrConflictState) {
		t.Fatalf("expected ErrConflictState, got %v", err)
	}
}

func TestUpdateBookingLimitAdminOnly(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	err := svc.UpdateBookingLimit(context.Background(), domain.RoleUser, 5)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestBuildReportTotals(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat1 := repo.mustCreateSeat("A1")
	seat2 := repo.mustCreateSeat("A2")

	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat1.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat1.ID,
		StartAt: now.Add(4 * time.Hour),
		EndAt:   now.Add(5 * time.Hour),
		Status:  domain.BookingStatusCanceled,
	})
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat2.ID,
		StartAt: now.Add(6 * time.Hour),
		EndAt:   now.Add(7 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	report, err := svc.BuildReport(context.Background(), domain.RoleAdmin, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalBookings != 3 {
		t.Fatalf("expected 3 total bookings, got %d", report.TotalBookings)
	}
	if report.CanceledBookings != 1 {
		t.Fatalf("expected 1 canceled booking, got %d", report.CanceledBookings)
	}
	if report.BySeat[seat1.ID] != 2 {
		t.Fatalf("expected seat1 count 2, got %d", report.BySeat[seat1.ID])
	}
	if report.BySeat[seat2.ID] != 1 {
		t.Fatalf("expected seat2 count 1, got %d", report.BySeat[seat2.ID])
	}
}

func TestRegisterLoginAndAuthenticateFlow(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user, err := svc.Register(context.Background(), RegisterInput{
		Name:     "User One",
		Email:    "user@mai.ru",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	token, err := svc.Login(context.Background(), LoginInput{
		Email:    "user@mai.ru",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	authUser, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if authUser.ID != user.ID {
		t.Fatalf("expected user id %d, got %d", user.ID, authUser.ID)
	}
}

func TestLoginFailsForWrongPassword(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	_, err := svc.Register(context.Background(), RegisterInput{
		Name:     "User One",
		Email:    "user@mai.ru",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Email:    "user@mai.ru",
		Password: "wrong-password",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestListAvailableSeatsExcludesInactiveAndConflicted(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	freeSeat := repo.mustCreateSeat("A1")
	conflictedSeat := repo.mustCreateSeat("A2")
	inactiveSeat := repo.mustCreateSeat("A3")
	inactiveSeat.Active = false
	_ = repo.UpdateSeat(context.Background(), &inactiveSeat)

	windowStart := now.Add(2 * time.Hour)
	windowEnd := now.Add(3 * time.Hour)
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  conflictedSeat.ID,
		StartAt: windowStart.Add(15 * time.Minute),
		EndAt:   windowEnd.Add(15 * time.Minute),
		Status:  domain.BookingStatusConfirmed,
	})

	seats, err := svc.ListAvailableSeats(context.Background(), windowStart, windowEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(seats) != 1 || seats[0].ID != freeSeat.ID {
		t.Fatalf("expected only seat %d available, got %#v", freeSeat.ID, seats)
	}
}

func TestCreateUpdateDeleteSeatLifecycleForAdmin(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	if _, err := svc.CreateSeat(context.Background(), domain.RoleUser, SeatInput{
		Name: "X1", Zone: "X", Type: "desk", Active: true,
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	seat, err := svc.CreateSeat(context.Background(), domain.RoleAdmin, SeatInput{
		Name: "X1", Zone: "X", Type: "desk", Active: true,
	})
	if err != nil {
		t.Fatalf("create seat failed: %v", err)
	}

	updated, err := svc.UpdateSeat(context.Background(), domain.RoleAdmin, seat.ID, SeatInput{
		Name: "X1-upd", Zone: "Y", Type: "focus", Active: true,
	})
	if err != nil {
		t.Fatalf("update seat failed: %v", err)
	}
	if updated.Name != "X1-upd" || updated.Zone != "Y" || updated.Type != "focus" {
		t.Fatalf("unexpected seat state after update: %#v", updated)
	}

	if err := svc.DeleteSeat(context.Background(), domain.RoleAdmin, seat.ID); err != nil {
		t.Fatalf("delete seat failed: %v", err)
	}
	if _, err := repo.GetSeatByID(context.Background(), seat.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected deleted seat to be missing, got %v", err)
	}
}

func TestAdminUpdateBookingStatus(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	booking := repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	if err := svc.AdminUpdateBookingStatus(context.Background(), domain.RoleUser, booking.ID, domain.BookingStatusCanceled); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	if err := svc.AdminUpdateBookingStatus(context.Background(), domain.RoleAdmin, booking.ID, domain.BookingStatusCanceled); err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	if err := svc.AdminUpdateBookingStatus(context.Background(), domain.RoleAdmin, booking.ID, domain.BookingStatusCompleted); !errors.Is(err, domain.ErrConflictState) {
		t.Fatalf("expected ErrConflictState on second terminal update, got %v", err)
	}
}

func TestUpdateBookingLimitValidation(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	if err := svc.UpdateBookingLimit(context.Background(), domain.RoleAdmin, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for zero, got %v", err)
	}
	if err := svc.UpdateBookingLimit(context.Background(), domain.RoleAdmin, 101); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for >100, got %v", err)
	}
	if err := svc.UpdateBookingLimit(context.Background(), domain.RoleAdmin, 7); err != nil {
		t.Fatalf("unexpected error updating limit: %v", err)
	}
	if repo.settings.BookingLimit != 7 {
		t.Fatalf("expected repo limit 7, got %d", repo.settings.BookingLimit)
	}
}

func TestAdminListBookingsRoleGuard(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("user@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	if _, err := svc.AdminListBookings(context.Background(), domain.RoleUser); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	all, err := svc.AdminListBookings(context.Background(), domain.RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 booking, got %d", len(all))
	}
}

type fakeRepo struct {
	users         map[int64]domain.User
	usersByEmail  map[string]int64
	sessions      map[string]domain.Session
	seats         map[int64]domain.Seat
	bookings      map[int64]domain.Booking
	settings      domain.Settings
	nextUserID    int64
	nextSeatID    int64
	nextBookingID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:         map[int64]domain.User{},
		usersByEmail:  map[string]int64{},
		sessions:      map[string]domain.Session{},
		seats:         map[int64]domain.Seat{},
		bookings:      map[int64]domain.Booking{},
		settings:      domain.Settings{BookingLimit: 3},
		nextUserID:    1,
		nextSeatID:    1,
		nextBookingID: 1,
	}
}

func (r *fakeRepo) mustCreateUser(email string, role domain.Role) domain.User {
	user := domain.User{
		Name:         "User",
		Email:        email,
		PasswordHash: "hash",
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}
	_ = r.CreateUser(context.Background(), &user)
	return user
}

func (r *fakeRepo) mustCreateSeat(name string) domain.Seat {
	seat := domain.Seat{
		Name:      name,
		Zone:      "A",
		Type:      "desk",
		Active:    true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_ = r.CreateSeat(context.Background(), &seat)
	return seat
}

func (r *fakeRepo) mustCreateBooking(booking domain.Booking) domain.Booking {
	booking.CreatedAt = time.Now().UTC()
	booking.UpdatedAt = booking.CreatedAt
	_ = r.CreateBooking(context.Background(), &booking)
	return booking
}

func (r *fakeRepo) CreateUser(_ context.Context, user *domain.User) error {
	if _, ok := r.usersByEmail[user.Email]; ok {
		return domain.ErrEmailTaken
	}
	user.ID = r.nextUserID
	r.nextUserID++
	r.users[user.ID] = *user
	r.usersByEmail[user.Email] = user.ID
	return nil
}

func (r *fakeRepo) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	id, ok := r.usersByEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	user := r.users[id]
	return &user, nil
}

func (r *fakeRepo) GetUserByID(_ context.Context, id int64) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (r *fakeRepo) CreateSession(_ context.Context, session *domain.Session) error {
	r.sessions[session.Token] = *session
	return nil
}

func (r *fakeRepo) GetSessionByToken(_ context.Context, token string) (*domain.Session, error) {
	session, ok := r.sessions[token]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &session, nil
}

func (r *fakeRepo) ListSeats(_ context.Context) ([]domain.Seat, error) {
	out := make([]domain.Seat, 0, len(r.seats))
	for _, seat := range r.seats {
		out = append(out, seat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeRepo) GetSeatByID(_ context.Context, id int64) (*domain.Seat, error) {
	seat, ok := r.seats[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &seat, nil
}

func (r *fakeRepo) CreateSeat(_ context.Context, seat *domain.Seat) error {
	seat.ID = r.nextSeatID
	r.nextSeatID++
	r.seats[seat.ID] = *seat
	return nil
}

func (r *fakeRepo) UpdateSeat(_ context.Context, seat *domain.Seat) error {
	if _, ok := r.seats[seat.ID]; !ok {
		return domain.ErrNotFound
	}
	r.seats[seat.ID] = *seat
	return nil
}

func (r *fakeRepo) DeleteSeat(_ context.Context, id int64) error {
	if _, ok := r.seats[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.seats, id)
	return nil
}

func (r *fakeRepo) HasFutureBookingsForSeat(_ context.Context, seatID int64, from time.Time) (bool, error) {
	for _, b := range r.bookings {
		if b.SeatID == seatID && b.Status == domain.BookingStatusConfirmed && b.StartAt.After(from) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) CreateBooking(_ context.Context, booking *domain.Booking) error {
	booking.ID = r.nextBookingID
	r.nextBookingID++
	r.bookings[booking.ID] = *booking
	return nil
}

func (r *fakeRepo) GetBookingByID(_ context.Context, id int64) (*domain.Booking, error) {
	booking, ok := r.bookings[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &booking, nil
}

func (r *fakeRepo) ListBookingsByUser(_ context.Context, userID int64) ([]domain.Booking, error) {
	out := make([]domain.Booking, 0)
	for _, booking := range r.bookings {
		if booking.UserID == userID {
			out = append(out, booking)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}

func (r *fakeRepo) ListAllBookings(_ context.Context) ([]domain.Booking, error) {
	out := make([]domain.Booking, 0, len(r.bookings))
	for _, booking := range r.bookings {
		out = append(out, booking)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeRepo) UpdateBookingStatus(_ context.Context, id int64, status domain.BookingStatus, updatedAt time.Time) error {
	booking, ok := r.bookings[id]
	if !ok {
		return domain.ErrNotFound
	}
	booking.Status = status
	booking.UpdatedAt = updatedAt
	r.bookings[id] = booking
	return nil
}

func overlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && aEnd.After(bStart)
}

func (r *fakeRepo) SeatHasConflict(_ context.Context, seatID int64, startAt, endAt time.Time) (bool, error) {
	for _, booking := range r.bookings {
		if booking.SeatID != seatID {
			continue
		}
		if booking.Status != domain.BookingStatusConfirmed {
			continue
		}
		if overlap(startAt, endAt, booking.StartAt, booking.EndAt) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) CountActiveBookingsByUser(_ context.Context, userID int64, now time.Time) (int, error) {
	count := 0
	for _, booking := range r.bookings {
		if booking.UserID != userID {
			continue
		}
		if booking.Status != domain.BookingStatusConfirmed {
			continue
		}
		if booking.EndAt.After(now) {
			count++
		}
	}
	return count, nil
}

func (r *fakeRepo) GetSettings(_ context.Context) (domain.Settings, error) {
	return r.settings, nil
}

func (r *fakeRepo) SetBookingLimit(_ context.Context, limit int) error {
	r.settings.BookingLimit = limit
	return nil
}
