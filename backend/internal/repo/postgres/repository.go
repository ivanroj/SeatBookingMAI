package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (name, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	if err := r.db.QueryRowContext(
		ctx,
		query,
		user.Name,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		user.CreatedAt,
	).Scan(&user.ID); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailTaken
		}
		return err
	}
	return nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, name, email, password_hash, role, created_at
		FROM users
		WHERE email = $1`

	user := domain.User{}
	if err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	const query = `
		SELECT id, name, email, password_hash, role, created_at
		FROM users
		WHERE id = $1`

	user := domain.User{}
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateSession(ctx context.Context, session *domain.Session) error {
	const query = `
		INSERT INTO sessions (token, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, session.Token, session.UserID, session.ExpiresAt, session.CreatedAt)
	return err
}

func (r *Repository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	const query = `
		SELECT token, user_id, expires_at, created_at
		FROM sessions
		WHERE token = $1`
	session := domain.Session{}
	if err := r.db.QueryRowContext(ctx, query, token).Scan(
		&session.Token,
		&session.UserID,
		&session.ExpiresAt,
		&session.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &session, nil
}

func (r *Repository) ListSeats(ctx context.Context) ([]domain.Seat, error) {
	const query = `
		SELECT id, name, zone, type, active, created_at, updated_at
		FROM seats
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seats := make([]domain.Seat, 0)
	for rows.Next() {
		seat := domain.Seat{}
		if err := rows.Scan(&seat.ID, &seat.Name, &seat.Zone, &seat.Type, &seat.Active, &seat.CreatedAt, &seat.UpdatedAt); err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

func (r *Repository) GetSeatByID(ctx context.Context, id int64) (*domain.Seat, error) {
	const query = `
		SELECT id, name, zone, type, active, created_at, updated_at
		FROM seats
		WHERE id = $1`
	seat := domain.Seat{}
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&seat.ID,
		&seat.Name,
		&seat.Zone,
		&seat.Type,
		&seat.Active,
		&seat.CreatedAt,
		&seat.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &seat, nil
}

func (r *Repository) CreateSeat(ctx context.Context, seat *domain.Seat) error {
	const query = `
		INSERT INTO seats (name, zone, type, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	return r.db.QueryRowContext(
		ctx,
		query,
		seat.Name,
		seat.Zone,
		seat.Type,
		seat.Active,
		seat.CreatedAt,
		seat.UpdatedAt,
	).Scan(&seat.ID)
}

func (r *Repository) UpdateSeat(ctx context.Context, seat *domain.Seat) error {
	const query = `
		UPDATE seats
		SET name = $1, zone = $2, type = $3, active = $4, updated_at = $5
		WHERE id = $6`
	result, err := r.db.ExecContext(ctx, query, seat.Name, seat.Zone, seat.Type, seat.Active, seat.UpdatedAt, seat.ID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteSeat(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM seats WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) HasFutureBookingsForSeat(ctx context.Context, seatID int64, from time.Time) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE seat_id = $1 AND status = $2 AND start_at > $3
		)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, seatID, string(domain.BookingStatusConfirmed), from).Scan(&exists)
	return exists, err
}

func (r *Repository) CreateBooking(ctx context.Context, booking *domain.Booking) error {
	const query = `
		INSERT INTO bookings (user_id, seat_id, start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	if err := r.db.QueryRowContext(
		ctx,
		query,
		booking.UserID,
		booking.SeatID,
		booking.StartAt,
		booking.EndAt,
		string(booking.Status),
		booking.CreatedAt,
		booking.UpdatedAt,
	).Scan(&booking.ID); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23P01" {
			return domain.ErrSeatUnavailable
		}
		return err
	}
	return nil
}

func (r *Repository) GetBookingByID(ctx context.Context, id int64) (*domain.Booking, error) {
	const query = `
		SELECT id, user_id, seat_id, start_at, end_at, status, created_at, updated_at
		FROM bookings
		WHERE id = $1`
	booking := domain.Booking{}
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&booking.ID,
		&booking.UserID,
		&booking.SeatID,
		&booking.StartAt,
		&booking.EndAt,
		&booking.Status,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &booking, nil
}

func (r *Repository) ListBookingsByUser(ctx context.Context, userID int64) ([]domain.Booking, error) {
	const query = `
		SELECT id, user_id, seat_id, start_at, end_at, status, created_at, updated_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY start_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]domain.Booking, 0)
	for rows.Next() {
		booking := domain.Booking{}
		if err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.SeatID,
			&booking.StartAt,
			&booking.EndAt,
			&booking.Status,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		); err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (r *Repository) ListAllBookings(ctx context.Context) ([]domain.Booking, error) {
	const query = `
		SELECT id, user_id, seat_id, start_at, end_at, status, created_at, updated_at
		FROM bookings
		ORDER BY start_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]domain.Booking, 0)
	for rows.Next() {
		booking := domain.Booking{}
		if err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.SeatID,
			&booking.StartAt,
			&booking.EndAt,
			&booking.Status,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		); err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (r *Repository) UpdateBookingStatus(ctx context.Context, id int64, status domain.BookingStatus, updatedAt time.Time) error {
	const query = `UPDATE bookings SET status = $1, updated_at = $2 WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, string(status), updatedAt, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) SeatHasConflict(ctx context.Context, seatID int64, startAt, endAt time.Time) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE seat_id = $1
			  AND status = $2
			  AND start_at < $4
			  AND end_at > $3
		)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, seatID, string(domain.BookingStatusConfirmed), startAt, endAt).Scan(&exists)
	return exists, err
}

func (r *Repository) CountActiveBookingsByUser(ctx context.Context, userID int64, now time.Time) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM bookings
		WHERE user_id = $1
		  AND status = $2
		  AND end_at > $3`
	var count int
	err := r.db.QueryRowContext(ctx, query, userID, string(domain.BookingStatusConfirmed), now).Scan(&count)
	return count, err
}

func (r *Repository) GetSettings(ctx context.Context) (domain.Settings, error) {
	const query = `SELECT booking_limit FROM settings WHERE id = 1`
	var settings domain.Settings
	err := r.db.QueryRowContext(ctx, query).Scan(&settings.BookingLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Settings{BookingLimit: 3}, nil
	}
	return settings, err
}

func (r *Repository) SetBookingLimit(ctx context.Context, limit int) error {
	const query = `
		INSERT INTO settings (id, booking_limit)
		VALUES (1, $1)
		ON CONFLICT (id)
		DO UPDATE SET booking_limit = EXCLUDED.booking_limit`
	_, err := r.db.ExecContext(ctx, query, limit)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (r *Repository) String() string {
	return fmt.Sprintf("postgres.Repository(%p)", r)
}
