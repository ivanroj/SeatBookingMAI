package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mitrich772/SeatBookingMAI/backend/internal/app"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/app/apptest"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/httpapi"
)

type harness struct {
	t      *testing.T
	repo   *apptest.FakeRepo
	svc    *app.Service
	server *httptest.Server
	now    time.Time
}

func newHarness(t *testing.T, opts ...app.Option) *harness {
	t.Helper()
	repo := apptest.NewFakeRepo()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	defaultOpts := []app.Option{
		app.WithClock(func() time.Time { return now }),
		app.WithCancelLeadTime(time.Hour),
	}
	defaultOpts = append(defaultOpts, opts...)
	svc := app.NewService(repo, defaultOpts...)
	server := httptest.NewServer(httpapi.NewHandler(svc))
	t.Cleanup(server.Close)
	return &harness{t: t, repo: repo, svc: svc, server: server, now: now}
}

func (h *harness) request(method, path, token string, body any) *http.Response {
	h.t.Helper()
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, rdr)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("do request: %v", err)
	}
	return resp
}

func (h *harness) decode(resp *http.Response, dst any) {
	h.t.Helper()
	defer resp.Body.Close()
	if dst == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		h.t.Fatalf("decode body: %v", err)
	}
}

func (h *harness) loginAs(email, password string) string {
	h.t.Helper()
	resp := h.request(http.MethodPost, "/api/auth/login", "", map[string]string{
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		h.t.Fatalf("login %s failed: %d %s", email, resp.StatusCode, string(body))
	}
	var out struct {
		Token string `json:"token"`
	}
	h.decode(resp, &out)
	return out.Token
}

func (h *harness) registerAndLogin(name, email, password string) (domain.User, string) {
	h.t.Helper()
	resp := h.request(http.MethodPost, "/api/auth/register", "", map[string]string{
		"name":     name,
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		h.t.Fatalf("register %s failed: %d %s", email, resp.StatusCode, string(body))
	}
	var user domain.User
	h.decode(resp, &user)
	return user, h.loginAs(email, password)
}

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	resp := h.request(http.MethodGet, "/healthz", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]string
	h.decode(resp, &out)
	if out["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", out["status"])
	}
}

func TestRegisterDuplicateEmailReturns409(t *testing.T) {
	h := newHarness(t)
	body := map[string]string{"name": "User", "email": "dup@mai.ru", "password": "secret123"}
	first := h.request(http.MethodPost, "/api/auth/register", "", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", first.StatusCode)
	}
	first.Body.Close()
	second := h.request(http.MethodPost, "/api/auth/register", "", body)
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", second.StatusCode)
	}
	second.Body.Close()
}

func TestRegisterInvalidEmailReturns400(t *testing.T) {
	h := newHarness(t)
	resp := h.request(http.MethodPost, "/api/auth/register", "", map[string]string{
		"name": "x", "email": "not-an-email", "password": "secret123",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLoginWrongPasswordReturns401(t *testing.T) {
	h := newHarness(t)
	h.repo.MustCreateUser("u@mai.ru", domain.RoleUser)

	// Repo's MustCreateUser stores a fixed "hash" string that bcrypt cannot verify;
	// for HTTP login we register through the API to get a real bcrypt hash.
	register := h.request(http.MethodPost, "/api/auth/register", "", map[string]string{
		"name": "User", "email": "alice@mai.ru", "password": "secret123",
	})
	register.Body.Close()

	resp := h.request(http.MethodPost, "/api/auth/login", "", map[string]string{
		"email": "alice@mai.ru", "password": "wrong-password",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestProtectedEndpointsRequireAuth(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/seats/available?start_at=2026-05-01T12:00:00Z&end_at=2026-05-01T13:00:00Z"},
		{http.MethodPost, "/api/bookings"},
		{http.MethodGet, "/api/bookings/me"},
		{http.MethodDelete, "/api/bookings/1"},
	}
	for _, tc := range cases {
		resp := h.request(tc.method, tc.path, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", tc.method, tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAdminEndpointsRequireAdmin(t *testing.T) {
	h := newHarness(t)
	_, token := h.registerAndLogin("Bob", "bob@mai.ru", "secret123")

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/admin/seats"},
		{http.MethodPut, "/api/admin/seats/1"},
		{http.MethodDelete, "/api/admin/seats/1"},
		{http.MethodGet, "/api/admin/bookings"},
		{http.MethodPatch, "/api/admin/bookings/1"},
		{http.MethodPut, "/api/admin/settings/limit"},
		{http.MethodGet, "/api/admin/reports?from=2026-05-01T00:00:00Z&to=2026-05-02T00:00:00Z"},
	}
	for _, tc := range cases {
		resp := h.request(tc.method, tc.path, token, map[string]string{})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d", tc.method, tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	h := newHarness(t)
	user, token := h.registerAndLogin("Alice", "alice@mai.ru", "secret123")

	resp := h.request(http.MethodGet, "/api/auth/me", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got domain.User
	h.decode(resp, &got)
	if got.ID != user.ID || got.Email != user.Email {
		t.Fatalf("unexpected /me response: %#v", got)
	}
	if got.PasswordHash != "" {
		t.Fatalf("password hash should never be serialized: %q", got.PasswordHash)
	}
}

func TestBookingFlowAndCancellation(t *testing.T) {
	h := newHarness(t)
	user, token := h.registerAndLogin("Carol", "carol@mai.ru", "secret123")
	seat := h.repo.MustCreateSeat("A-100")

	startAt := h.now.Add(2 * time.Hour)
	endAt := h.now.Add(3 * time.Hour)

	// Create
	resp := h.request(http.MethodPost, "/api/bookings", token, map[string]any{
		"seat_id":  seat.ID,
		"start_at": startAt.Format(time.RFC3339),
		"end_at":   endAt.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create booking: expected 201, got %d", resp.StatusCode)
	}
	var booking domain.Booking
	h.decode(resp, &booking)
	if booking.UserID != user.ID || booking.SeatID != seat.ID || booking.Status != domain.BookingStatusConfirmed {
		t.Fatalf("unexpected booking: %#v", booking)
	}

	// List my bookings
	resp = h.request(http.MethodGet, "/api/bookings/me", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list bookings: expected 200, got %d", resp.StatusCode)
	}
	var bookings []domain.Booking
	h.decode(resp, &bookings)
	if len(bookings) != 1 || bookings[0].ID != booking.ID {
		t.Fatalf("expected 1 booking, got %#v", bookings)
	}

	// Cancel
	resp = h.request(http.MethodDelete, fmt.Sprintf("/api/bookings/%d", booking.ID), token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel booking: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	stored, err := h.repo.GetBookingByID(context.Background(), booking.ID)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if stored.Status != domain.BookingStatusCanceled {
		t.Fatalf("expected canceled, got %s", stored.Status)
	}
}

func TestAvailableSeatsRequiresValidWindow(t *testing.T) {
	h := newHarness(t)
	_, token := h.registerAndLogin("Dan", "dan@mai.ru", "secret123")

	// Missing parameters → 400
	resp := h.request(http.MethodGet, "/api/seats/available", token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// End before start → 400
	bad := url.Values{}
	bad.Set("start_at", h.now.Add(2*time.Hour).Format(time.RFC3339))
	bad.Set("end_at", h.now.Add(time.Hour).Format(time.RFC3339))
	resp = h.request(http.MethodGet, "/api/seats/available?"+bad.Encode(), token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAvailableSeatsExcludesConflicts(t *testing.T) {
	h := newHarness(t)
	_, token := h.registerAndLogin("Eve", "eve@mai.ru", "secret123")
	free := h.repo.MustCreateSeat("FREE")
	taken := h.repo.MustCreateSeat("TAKEN")

	startAt := h.now.Add(2 * time.Hour)
	endAt := h.now.Add(3 * time.Hour)

	owner := h.repo.MustCreateUser("owner@mai.ru", domain.RoleUser)
	h.repo.MustCreateBooking(domain.Booking{
		UserID:  owner.ID,
		SeatID:  taken.ID,
		StartAt: startAt.Add(15 * time.Minute),
		EndAt:   endAt.Add(15 * time.Minute),
		Status:  domain.BookingStatusConfirmed,
	})

	q := url.Values{}
	q.Set("start_at", startAt.Format(time.RFC3339))
	q.Set("end_at", endAt.Format(time.RFC3339))
	resp := h.request(http.MethodGet, "/api/seats/available?"+q.Encode(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var seats []domain.Seat
	h.decode(resp, &seats)
	if len(seats) != 1 || seats[0].ID != free.ID {
		t.Fatalf("expected only free seat, got %#v", seats)
	}
}

func TestAdminSeatLifecycle(t *testing.T) {
	h := newHarness(t)
	// Bootstrap admin: create directly in repo (bypassing register) and then issue
	// a session via the service so the bearer token works.
	admin := h.repo.MustCreateUser("admin@mai.ru", domain.RoleAdmin)
	token := issueToken(t, h, admin.ID)

	// Create
	resp := h.request(http.MethodPost, "/api/admin/seats", token, map[string]any{
		"name": "X-1", "zone": "X", "type": "desk", "active": true,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create seat: expected 201, got %d", resp.StatusCode)
	}
	var seat domain.Seat
	h.decode(resp, &seat)
	if seat.Name != "X-1" {
		t.Fatalf("unexpected seat: %#v", seat)
	}

	// Update
	resp = h.request(http.MethodPut, fmt.Sprintf("/api/admin/seats/%d", seat.ID), token, map[string]any{
		"name": "X-1-upd", "zone": "Y", "type": "focus", "active": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update seat: expected 200, got %d", resp.StatusCode)
	}
	var updated domain.Seat
	h.decode(resp, &updated)
	if updated.Name != "X-1-upd" || updated.Zone != "Y" || updated.Type != "focus" || updated.Active {
		t.Fatalf("unexpected update result: %#v", updated)
	}

	// Delete
	resp = h.request(http.MethodDelete, fmt.Sprintf("/api/admin/seats/%d", seat.ID), token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete seat: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	if _, err := h.repo.GetSeatByID(context.Background(), seat.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected seat to be deleted, got %v", err)
	}
}

func TestAdminUpdateBookingAndLimit(t *testing.T) {
	h := newHarness(t)
	admin := h.repo.MustCreateUser("admin@mai.ru", domain.RoleAdmin)
	token := issueToken(t, h, admin.ID)

	owner := h.repo.MustCreateUser("owner@mai.ru", domain.RoleUser)
	seat := h.repo.MustCreateSeat("A-1")
	booking := h.repo.MustCreateBooking(domain.Booking{
		UserID:  owner.ID,
		SeatID:  seat.ID,
		StartAt: h.now.Add(2 * time.Hour),
		EndAt:   h.now.Add(3 * time.Hour),
		Status:  domain.BookingStatusConfirmed,
	})

	// Update booking status
	resp := h.request(http.MethodPatch, fmt.Sprintf("/api/admin/bookings/%d", booking.ID), token, map[string]string{
		"status": string(domain.BookingStatusCompleted),
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("admin update booking: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Update limit invalid
	resp = h.request(http.MethodPut, "/api/admin/settings/limit", token, map[string]int{"limit": 0})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("admin update limit invalid: expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Update limit valid
	resp = h.request(http.MethodPut, "/api/admin/settings/limit", token, map[string]int{"limit": 7})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("admin update limit: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if got := h.repo.CurrentBookingLimit(); got != 7 {
		t.Fatalf("expected limit 7 in repo, got %d", got)
	}

	// List all bookings
	resp = h.request(http.MethodGet, "/api/admin/bookings", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin list bookings: expected 200, got %d", resp.StatusCode)
	}
	var bookings []domain.Booking
	h.decode(resp, &bookings)
	if len(bookings) != 1 {
		t.Fatalf("expected 1 booking, got %d", len(bookings))
	}
}

func TestAdminReportTotals(t *testing.T) {
	h := newHarness(t)
	admin := h.repo.MustCreateUser("admin@mai.ru", domain.RoleAdmin)
	token := issueToken(t, h, admin.ID)

	owner := h.repo.MustCreateUser("u@mai.ru", domain.RoleUser)
	seat := h.repo.MustCreateSeat("A-1")
	h.repo.MustCreateBooking(domain.Booking{
		UserID: owner.ID, SeatID: seat.ID,
		StartAt: h.now.Add(2 * time.Hour), EndAt: h.now.Add(3 * time.Hour),
		Status: domain.BookingStatusConfirmed,
	})
	h.repo.MustCreateBooking(domain.Booking{
		UserID: owner.ID, SeatID: seat.ID,
		StartAt: h.now.Add(4 * time.Hour), EndAt: h.now.Add(5 * time.Hour),
		Status: domain.BookingStatusCanceled,
	})

	q := url.Values{}
	q.Set("from", h.now.Format(time.RFC3339))
	q.Set("to", h.now.Add(24*time.Hour).Format(time.RFC3339))

	resp := h.request(http.MethodGet, "/api/admin/reports?"+q.Encode(), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var report struct {
		TotalBookings    int            `json:"total_bookings"`
		CanceledBookings int            `json:"canceled_bookings"`
		BySeat           map[string]int `json:"by_seat"`
	}
	h.decode(resp, &report)
	if report.TotalBookings != 2 {
		t.Fatalf("expected 2 total, got %d", report.TotalBookings)
	}
	if report.CanceledBookings != 1 {
		t.Fatalf("expected 1 canceled, got %d", report.CanceledBookings)
	}
}

func TestRequestWithUnknownFieldsReturns400(t *testing.T) {
	h := newHarness(t)
	resp := h.request(http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":     "user",
		"email":    "x@mai.ru",
		"password": "secret123",
		"unknown":  "field",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCorsPreflight(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequest(http.MethodOptions, h.server.URL+"/api/auth/login", strings.NewReader(""))
	req.Header.Set("Origin", "https://example.com")
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight: expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("expected CORS header to be set")
	}
}

func TestDeviceLoginCreatesAnonymousStudent(t *testing.T) {
	h := newHarness(t)
	deviceID := "dev-aaaa-bbbb-cccc-dddd"

	first := h.request(http.MethodPost, "/api/auth/device", "", map[string]string{"device_id": deviceID})
	if first.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(first.Body)
		first.Body.Close()
		t.Fatalf("first device login: expected 200, got %d %s", first.StatusCode, string(body))
	}
	var firstResp struct {
		Token string `json:"token"`
	}
	h.decode(first, &firstResp)
	if firstResp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Second call with the same device id reuses the user (no email collision)
	// and just issues a new token.
	second := h.request(http.MethodPost, "/api/auth/device", "", map[string]string{"device_id": deviceID})
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second device login: expected 200, got %d", second.StatusCode)
	}
	var secondResp struct {
		Token string `json:"token"`
	}
	h.decode(second, &secondResp)
	if secondResp.Token == "" || secondResp.Token == firstResp.Token {
		t.Fatalf("expected fresh non-empty token, got %q (was %q)", secondResp.Token, firstResp.Token)
	}

	// /api/auth/me must report this hidden user with role=user and a device_id.
	me := h.request(http.MethodGet, "/api/auth/me", secondResp.Token, nil)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("me: expected 200, got %d", me.StatusCode)
	}
	var u domain.User
	h.decode(me, &u)
	if u.Role != domain.RoleUser {
		t.Fatalf("expected role=user, got %q", u.Role)
	}
	if u.DeviceID != deviceID {
		t.Fatalf("expected device_id to round-trip, got %q", u.DeviceID)
	}
}

func TestDeviceLoginRejectsShortDeviceID(t *testing.T) {
	h := newHarness(t)
	resp := h.request(http.MethodPost, "/api/auth/device", "", map[string]string{"device_id": "short"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStudentBookingStoresDisplayName(t *testing.T) {
	h := newHarness(t)
	deviceID := "dev-display-name-test-xxxx"
	dl := h.request(http.MethodPost, "/api/auth/device", "", map[string]string{"device_id": deviceID})
	if dl.StatusCode != http.StatusOK {
		t.Fatalf("device login: %d", dl.StatusCode)
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	h.decode(dl, &loginResp)

	seat := h.repo.MustCreateSeat("S-1")
	resp := h.request(http.MethodPost, "/api/bookings", loginResp.Token, map[string]any{
		"seat_id":      seat.ID,
		"start_at":     h.now.Add(2 * time.Hour).Format(time.RFC3339),
		"end_at":       h.now.Add(3 * time.Hour).Format(time.RFC3339),
		"display_name": "Иван Иванов",
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create booking: expected 201, got %d %s", resp.StatusCode, string(body))
	}
	var booking domain.Booking
	h.decode(resp, &booking)
	if booking.DisplayName != "Иван Иванов" {
		t.Fatalf("expected display_name to round-trip, got %q", booking.DisplayName)
	}

	// The hidden student user's name should follow the latest display name
	// so admin views are readable.
	me := h.request(http.MethodGet, "/api/auth/me", loginResp.Token, nil)
	var u domain.User
	h.decode(me, &u)
	if u.Name != "Иван Иванов" {
		t.Fatalf("expected user name to track display name, got %q", u.Name)
	}
}

// issueToken creates a session in the fake repo for the given user and returns
// the bearer token. This avoids relying on bcrypt-hashed seed passwords in the
// admin path of tests.
func issueToken(t *testing.T, h *harness, userID int64) string {
	t.Helper()
	token := fmt.Sprintf("test-token-%d-%d", userID, time.Now().UnixNano())
	if err := h.repo.CreateSession(context.Background(), &domain.Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: h.now.Add(24 * time.Hour),
		CreatedAt: h.now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}
