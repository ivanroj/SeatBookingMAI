package app

import (
	"context"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	GetUserByDeviceID(ctx context.Context, deviceID string) (*domain.User, error)
	UpdateUserName(ctx context.Context, id int64, name string) error

	CreateSession(ctx context.Context, session *domain.Session) error
	GetSessionByToken(ctx context.Context, token string) (*domain.Session, error)

	ListCoworkings(ctx context.Context) ([]domain.Coworking, error)
	GetCoworkingByID(ctx context.Context, id int64) (*domain.Coworking, error)
	CreateCoworking(ctx context.Context, c *domain.Coworking) error
	UpdateCoworking(ctx context.Context, c *domain.Coworking) error
	DeleteCoworking(ctx context.Context, id int64) error

	ListSeats(ctx context.Context, coworkingID int64) ([]domain.Seat, error)
	GetSeatByID(ctx context.Context, id int64) (*domain.Seat, error)
	CreateSeat(ctx context.Context, seat *domain.Seat) error
	UpdateSeat(ctx context.Context, seat *domain.Seat) error
	DeleteSeat(ctx context.Context, id int64) error
	HasFutureBookingsForSeat(ctx context.Context, seatID int64, from time.Time) (bool, error)

	CreateBooking(ctx context.Context, booking *domain.Booking) error
	GetBookingByID(ctx context.Context, id int64) (*domain.Booking, error)
	ListBookingsByUser(ctx context.Context, userID int64) ([]domain.Booking, error)
	ListAllBookings(ctx context.Context) ([]domain.Booking, error)
	UpdateBookingStatus(ctx context.Context, id int64, status domain.BookingStatus, updatedAt time.Time) error
	SeatHasConflict(ctx context.Context, seatID int64, startAt, endAt time.Time) (bool, error)
	CountActiveBookingsByUser(ctx context.Context, userID int64, now time.Time) (int, error)

	GetSettings(ctx context.Context) (domain.Settings, error)
	SetBookingLimit(ctx context.Context, limit int) error
}
