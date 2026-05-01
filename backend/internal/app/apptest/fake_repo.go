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

	users        map[int64]domain.User
	usersByEmail map[string]int64
	sessions     map[string]domain.Session
	seats        map[int64]domain.Seat
	bookings     map[int64]domain.Booking
	settings     domain.Settings

	nextUserID    int64
	nextSeatID    int64
	nextBookingID int64
}

// NewFakeRepo returns an empty repo with a default booking limit of 3.
func NewFakeRepo() *FakeRepo {
	return &FakeRepo{
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

// MustCreateSeat inserts an active "desk" seat in zone A.
func (r *FakeRepo) MustCreateSeat(name string) domain.Seat {
	seat := domain.Seat{
		Name:      name,
		Zone:      "A",
		Type:      "desk",
		Active:    true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
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
	user.ID = r.nextUserID
	r.nextUserID++
	r.users[user.ID] = *user
	r.usersByEmail[user.Email] = user.ID
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

func (r *FakeRepo) ListSeats(_ context.Context) ([]domain.Seat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Seat, 0, len(r.seats))
	for _, seat := range r.seats {
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
