// Package apptest provides an in-memory implementation of app.Repository for
// use in tests across the codebase. It is intentionally small and mirrors the
// behaviour of the production postgres repository: unique-email enforcement,
// EXCLUDE-style overlap rejection on confirmed bookings, settings persistence,
// etc.
package apptest

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

// FakeRepo is a thread-safe in-memory implementation of app.Repository.
type FakeRepo struct {
	mu sync.Mutex

	users           map[int64]domain.User
	usersByEmail    map[string]int64
	usersByDeviceID map[string]int64
	sessions        map[string]domain.Session
	coworkings      map[int64]domain.Coworking
	coworkingsName  map[string]int64
	seats           map[int64]domain.Seat
	bookings        map[int64]domain.Booking
	settings        domain.Settings

	nextUserID      int64
	nextCoworkingID int64
	nextSeatID      int64
	nextBookingID   int64
}

// NewFakeRepo returns an empty repo with a default booking limit of 3 and a
// pre-seeded default coworking so existing tests that don't care about
// coworkings can still create seats with `MustCreateSeat`.
func NewFakeRepo() *FakeRepo {
	r := &FakeRepo{
		users:           map[int64]domain.User{},
		usersByEmail:    map[string]int64{},
		usersByDeviceID: map[string]int64{},
		sessions:        map[string]domain.Session{},
		coworkings:      map[int64]domain.Coworking{},
		coworkingsName:  map[string]int64{},
		seats:           map[int64]domain.Seat{},
		bookings:        map[int64]domain.Booking{},
		settings:        domain.Settings{BookingLimit: 3},
		nextUserID:      1,
		nextCoworkingID: 1,
		nextSeatID:      1,
		nextBookingID:   1,
	}
	defaultCw := domain.Coworking{
		Name:      "Default",
		Capacity:  100,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := r.CreateCoworking(context.Background(), &defaultCw); err != nil {
		panic(err)
	}
	return r
}

// DefaultCoworkingID returns the seeded default coworking id (=1) for tests
// that just want to throw a seat in somewhere.
func (r *FakeRepo) DefaultCoworkingID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return 1
}

// SetBookingLimitDirect bypasses the public API to seed the limit in tests.
func (r *FakeRepo) SetBookingLimitDirect(limit int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.BookingLimit = limit
}

// CurrentBookingLimit returns the current limit (for assertions in tests).
func (r *FakeRepo) CurrentBookingLimit() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings.BookingLimit
}

// MustCreateUser inserts a user with a fixed password hash and returns it.
func (r *FakeRepo) MustCreateUser(email string, role domain.Role) domain.User {
	user := domain.User{
		Name:         "Test " + email,
		Email:        email,
		PasswordHash: "hash",
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.CreateUser(context.Background(), &user); err != nil {
		panic(err)
	}
	return user
}

// MustCreateSeat inserts an active "desk" seat in zone A in the default
// coworking. The grid position is auto-assigned per call.
func (r *FakeRepo) MustCreateSeat(name string) domain.Seat {
	r.mu.Lock()
	x := r.nextSeatID - 1
	r.mu.Unlock()
	seat := domain.Seat{
		CoworkingID: 1,
		Name:        name,
		Zone:        "A",
		Type:        "desk",
		GridX:       int(x % 8),
		GridY:       int(x / 8),
		Active:      true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := r.CreateSeat(context.Background(), &seat); err != nil {
		panic(err)
	}
	return seat
}

// MustCreateBooking inserts a booking, filling timestamps if zero.
func (r *FakeRepo) MustCreateBooking(b domain.Booking) domain.Booking {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = b.CreatedAt
	}
	if err := r.CreateBooking(context.Background(), &b); err != nil {
		panic(err)
	}
	return b
}

func (r *FakeRepo) CreateUser(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.usersByEmail[user.Email]; ok {
		return domain.ErrEmailTaken
	}
	if user.DeviceID != "" {
		if _, ok := r.usersByDeviceID[user.DeviceID]; ok {
			return domain.ErrEmailTaken
		}
	}
	user.ID = r.nextUserID
	r.nextUserID++
	r.users[user.ID] = *user
	r.usersByEmail[user.Email] = user.ID
	if user.DeviceID != "" {
		r.usersByDeviceID[user.DeviceID] = user.ID
	}
	return nil
}

func (r *FakeRepo) GetUserByDeviceID(_ context.Context, deviceID string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.usersByDeviceID[deviceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	user := r.users[id]
	return &user, nil
}

func (r *FakeRepo) UpdateUserName(_ context.Context, id int64, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	user.Name = name
	r.users[id] = user
	return nil
}

func (r *FakeRepo) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.usersByEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	user := r.users[id]
	return &user, nil
}

func (r *FakeRepo) GetUserByID(_ context.Context, id int64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (r *FakeRepo) CreateSession(_ context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.Token] = *session
	return nil
}

func (r *FakeRepo) GetSessionByToken(_ context.Context, token string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[token]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &session, nil
}

func (r *FakeRepo) ListCoworkings(_ context.Context) ([]domain.Coworking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Coworking, 0, len(r.coworkings))
	for _, c := range r.coworkings {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *FakeRepo) GetCoworkingByID(_ context.Context, id int64) (*domain.Coworking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.coworkings[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &c, nil
}

func (r *FakeRepo) CreateCoworking(_ context.Context, c *domain.Coworking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.coworkingsName[c.Name]; ok {
		return domain.ErrConflictState
	}
	c.ID = r.nextCoworkingID
	r.nextCoworkingID++
	r.coworkings[c.ID] = *c
	r.coworkingsName[c.Name] = c.ID
	return nil
}

func (r *FakeRepo) UpdateCoworking(_ context.Context, c *domain.Coworking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.coworkings[c.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if c.Name != old.Name {
		if _, taken := r.coworkingsName[c.Name]; taken {
			return domain.ErrConflictState
		}
		delete(r.coworkingsName, old.Name)
		r.coworkingsName[c.Name] = c.ID
	}
	r.coworkings[c.ID] = *c
	return nil
}

func (r *FakeRepo) DeleteCoworking(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.coworkings[id]
	if !ok {
		return domain.ErrNotFound
	}
	deletedSeats := make(map[int64]struct{})
	for seatID, seat := range r.seats {
		if seat.CoworkingID == id {
			deletedSeats[seatID] = struct{}{}
			delete(r.seats, seatID)
		}
	}
	// Mirror the bookings_cascade migration: removing a seat removes its
	// bookings.
	for bookingID, booking := range r.bookings {
		if _, ok := deletedSeats[booking.SeatID]; ok {
			delete(r.bookings, bookingID)
		}
	}
	delete(r.coworkings, id)
	delete(r.coworkingsName, c.Name)
	return nil
}

func (r *FakeRepo) ListSeats(_ context.Context, coworkingID int64) ([]domain.Seat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Seat, 0, len(r.seats))
	for _, seat := range r.seats {
		if coworkingID > 0 && seat.CoworkingID != coworkingID {
			continue
		}
		out = append(out, seat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *FakeRepo) GetSeatByID(_ context.Context, id int64) (*domain.Seat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, ok := r.seats[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &seat, nil
}

func (r *FakeRepo) CreateSeat(_ context.Context, seat *domain.Seat) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.seats {
		if existing.CoworkingID == seat.CoworkingID && existing.GridX == seat.GridX && existing.GridY == seat.GridY {
			return domain.ErrConflictState
		}
		if existing.CoworkingID == seat.CoworkingID && existing.Name == seat.Name {
			return domain.ErrConflictState
		}
	}
	seat.ID = r.nextSeatID
	r.nextSeatID++
	r.seats[seat.ID] = *seat
	return nil
}

func (r *FakeRepo) UpdateSeat(_ context.Context, seat *domain.Seat) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.seats[seat.ID]; !ok {
		return domain.ErrNotFound
	}
	for id, existing := range r.seats {
		if id == seat.ID {
			continue
		}
		if existing.CoworkingID == seat.CoworkingID && existing.GridX == seat.GridX && existing.GridY == seat.GridY {
			return domain.ErrConflictState
		}
		if existing.CoworkingID == seat.CoworkingID && existing.Name == seat.Name {
			return domain.ErrConflictState
		}
	}
	r.seats[seat.ID] = *seat
	return nil
}

func (r *FakeRepo) DeleteSeat(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.seats[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.seats, id)
	return nil
}

func (r *FakeRepo) HasFutureBookingsForSeat(_ context.Context, seatID int64, from time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.bookings {
		if b.SeatID == seatID && b.Status == domain.BookingStatusConfirmed && b.StartAt.After(from) {
			return true, nil
		}
	}
	return false, nil
}

func (r *FakeRepo) CreateBooking(_ context.Context, booking *domain.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	booking.ID = r.nextBookingID
	r.nextBookingID++
	r.bookings[booking.ID] = *booking
	return nil
}

func (r *FakeRepo) GetBookingByID(_ context.Context, id int64) (*domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	booking, ok := r.bookings[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &booking, nil
}

func (r *FakeRepo) ListBookingsByUser(_ context.Context, userID int64) ([]domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Booking, 0)
	for _, booking := range r.bookings {
		if booking.UserID == userID {
			out = append(out, booking)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}

func (r *FakeRepo) ListAllBookings(_ context.Context) ([]domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Booking, 0, len(r.bookings))
	for _, booking := range r.bookings {
		out = append(out, booking)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *FakeRepo) UpdateBookingStatus(_ context.Context, id int64, status domain.BookingStatus, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	booking, ok := r.bookings[id]
	if !ok {
		return domain.ErrNotFound
	}
	booking.Status = status
	booking.UpdatedAt = updatedAt
	r.bookings[id] = booking
	return nil
}

func (r *FakeRepo) SeatHasConflict(_ context.Context, seatID int64, startAt, endAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, booking := range r.bookings {
		if booking.SeatID != seatID {
			continue
		}
		if booking.Status != domain.BookingStatusConfirmed {
			continue
		}
		if startAt.Before(booking.EndAt) && endAt.After(booking.StartAt) {
			return true, nil
		}
	}
	return false, nil
}

func (r *FakeRepo) CountActiveBookingsByUser(_ context.Context, userID int64, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *FakeRepo) GetSettings(_ context.Context) (domain.Settings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings, nil
}

func (r *FakeRepo) SetBookingLimit(_ context.Context, limit int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.BookingLimit = limit
	return nil
}
