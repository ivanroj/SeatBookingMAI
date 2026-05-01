package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

func TestAuthenticateRejectsExpiredSession(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("expired@mai.ru", domain.RoleUser)
	token := "expired-token"
	repo.sessions[token] = domain.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: now.Add(-time.Minute), // already expired
		CreatedAt: now.Add(-2 * time.Hour),
	}

	if _, err := svc.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticateRejectsEmptyToken(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	if _, err := svc.Authenticate(context.Background(), "  "); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateBookingRejectsPastStart(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")

	_, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat.ID,
		StartAt: now.Add(-time.Hour),
		EndAt:   now.Add(time.Hour),
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateBookingRejectsZeroDuration(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")

	at := now.Add(2 * time.Hour)
	_, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat.ID,
		StartAt: at,
		EndAt:   at,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateBookingRejectsInactiveSeat(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	seat.Active = false
	if err := repo.UpdateSeat(context.Background(), &seat); err != nil {
		t.Fatalf("update seat: %v", err)
	}

	_, err := svc.CreateBooking(context.Background(), user.ID, CreateBookingInput{
		SeatID:  seat.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
	})
	if !errors.Is(err, domain.ErrSeatUnavailable) {
		t.Fatalf("expected ErrSeatUnavailable, got %v", err)
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	_, err := svc.Register(context.Background(), RegisterInput{
		Name:     "User",
		Email:    "not-an-email",
		Password: "secret123",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestBuildReportRejectsInvertedWindow(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	_, err := svc.BuildReport(context.Background(), domain.RoleAdmin, now.Add(time.Hour), now)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancelBookingRejectsAlreadyCanceled(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }), WithCancelLeadTime(time.Hour))

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	seat := repo.mustCreateSeat("A1")
	booking := repo.mustCreateBooking(domain.Booking{
		UserID: user.ID, SeatID: seat.ID,
		StartAt: now.Add(3 * time.Hour), EndAt: now.Add(4 * time.Hour),
		Status: domain.BookingStatusCanceled,
	})

	err := svc.CancelBooking(context.Background(), user.ID, booking.ID)
	if !errors.Is(err, domain.ErrConflictState) {
		t.Fatalf("expected ErrConflictState, got %v", err)
	}
}
