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

func TestCreateCoworkingValidatesInputAndRole(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	if _, err := svc.CreateCoworking(context.Background(), domain.RoleUser, CoworkingInput{Name: "X", Capacity: 5}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if _, err := svc.CreateCoworking(context.Background(), domain.RoleAdmin, CoworkingInput{Name: "", Capacity: 5}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}
	if _, err := svc.CreateCoworking(context.Background(), domain.RoleAdmin, CoworkingInput{Name: "X", Capacity: 0}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for zero capacity, got %v", err)
	}
	c, err := svc.CreateCoworking(context.Background(), domain.RoleAdmin, CoworkingInput{Name: "Yard", Capacity: 8})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.Name != "Yard" || c.Capacity != 8 {
		t.Fatalf("unexpected coworking: %#v", c)
	}
}

func TestLogsAdminOnly(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	svc.LogEvent("hello", map[string]any{"k": "v"})

	if _, err := svc.Logs(context.Background(), domain.RoleUser, 0); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("student got logs: %v", err)
	}
	logs, err := svc.Logs(context.Background(), domain.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("admin logs: %v", err)
	}
	if len(logs) == 0 || logs[0].Event != "hello" {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}

func TestListSeatsForMapMarksBusyAndInactive(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc := NewService(repo, WithClock(func() time.Time { return now }))

	user := repo.mustCreateUser("u@mai.ru", domain.RoleUser)
	free := repo.mustCreateSeat("A1")
	booked := repo.mustCreateSeat("A2")
	inactive := repo.mustCreateSeat("A3")
	inactive.Active = false
	if err := repo.UpdateSeat(context.Background(), &inactive); err != nil {
		t.Fatalf("update: %v", err)
	}
	repo.mustCreateBooking(domain.Booking{
		UserID:  user.ID,
		SeatID:  booked.ID,
		StartAt: now.Add(2 * time.Hour),
		EndAt:   now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	seats, err := svc.ListSeatsForMap(context.Background(), 1, now.Add(2*time.Hour+15*time.Minute), now.Add(2*time.Hour+45*time.Minute))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	state := map[int64]bool{}
	for _, s := range seats {
		state[s.ID] = s.IsBusy
	}
	if state[free.ID] != false || state[booked.ID] != true || state[inactive.ID] != true {
		t.Fatalf("unexpected map state: %#v", state)
	}
}
