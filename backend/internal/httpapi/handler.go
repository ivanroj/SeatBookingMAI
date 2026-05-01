package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/app"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/domain"
)

type contextKey string

const userContextKey contextKey = "auth_user"

type Handler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) http.Handler {
	h := &Handler{svc: svc}
	r := chi.NewRouter()
	r.Use(corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Post("/api/auth/register", h.register)
	r.Post("/api/auth/login", h.login)
	r.Post("/api/auth/device", h.deviceLogin)

	r.Group(func(g chi.Router) {
		g.Use(h.authMiddleware)
		g.Get("/api/auth/me", h.me)
		g.Get("/api/seats/available", h.availableSeats)
		g.Post("/api/bookings", h.createBooking)
		g.Get("/api/bookings/me", h.myBookings)
		g.Delete("/api/bookings/{id}", h.cancelBooking)
	})

	r.Group(func(g chi.Router) {
		g.Use(h.authMiddleware, h.adminMiddleware)
		g.Post("/api/admin/seats", h.createSeat)
		g.Put("/api/admin/seats/{id}", h.updateSeat)
		g.Delete("/api/admin/seats/{id}", h.deleteSeat)

		g.Get("/api/admin/bookings", h.adminListBookings)
		g.Patch("/api/admin/bookings/{id}", h.adminUpdateBooking)

		g.Put("/api/admin/settings/limit", h.updateLimit)
		g.Get("/api/admin/reports", h.report)
	})

	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	user, err := h.svc.Register(r.Context(), app.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	token, err := h.svc.Login(r.Context(), app.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) deviceLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	token, err := h.svc.LoginAsDevice(r.Context(), req.DeviceID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) availableSeats(w http.ResponseWriter, r *http.Request) {
	startAt, endAt, err := parseWindowQuery(r)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	seats, err := h.svc.ListAvailableSeats(r.Context(), startAt, endAt)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, seats)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) createBooking(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	var req struct {
		SeatID      int64  `json:"seat_id"`
		StartAt     string `json:"start_at"`
		EndAt       string `json:"end_at"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}

	booking, err := h.svc.CreateBooking(r.Context(), user.ID, app.CreateBookingInput{
		SeatID:      req.SeatID,
		StartAt:     startAt,
		EndAt:       endAt,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, booking)
}

func (h *Handler) myBookings(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	bookings, err := h.svc.ListUserBookings(r.Context(), user.ID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

func (h *Handler) cancelBooking(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	bookingID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	if err := h.svc.CancelBooking(r.Context(), user.ID, bookingID); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createSeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Zone   string `json:"zone"`
		Type   string `json:"type"`
		Active bool   `json:"active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	seat, err := h.svc.CreateSeat(r.Context(), domain.RoleAdmin, app.SeatInput{
		Name:   req.Name,
		Zone:   req.Zone,
		Type:   req.Type,
		Active: req.Active,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, seat)
}

func (h *Handler) updateSeat(w http.ResponseWriter, r *http.Request) {
	seatID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	var req struct {
		Name   string `json:"name"`
		Zone   string `json:"zone"`
		Type   string `json:"type"`
		Active bool   `json:"active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	seat, err := h.svc.UpdateSeat(r.Context(), domain.RoleAdmin, seatID, app.SeatInput{
		Name:   req.Name,
		Zone:   req.Zone,
		Type:   req.Type,
		Active: req.Active,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, seat)
}

func (h *Handler) deleteSeat(w http.ResponseWriter, r *http.Request) {
	seatID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	if err := h.svc.DeleteSeat(r.Context(), domain.RoleAdmin, seatID); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) adminListBookings(w http.ResponseWriter, r *http.Request) {
	bookings, err := h.svc.AdminListBookings(r.Context(), domain.RoleAdmin)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}

func (h *Handler) adminUpdateBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	var req struct {
		Status domain.BookingStatus `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	if err := h.svc.AdminUpdateBookingStatus(r.Context(), domain.RoleAdmin, bookingID, req.Status); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateLimit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Limit int `json:"limit"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	if err := h.svc.UpdateBookingLimit(r.Context(), domain.RoleAdmin, req.Limit); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil {
		writeAppError(w, domain.ErrInvalidInput)
		return
	}
	report, err := h.svc.BuildReport(r.Context(), domain.RoleAdmin, from, to)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeAppError(w, domain.ErrUnauthorized)
			return
		}
		user, err := h.svc.Authenticate(r.Context(), token)
		if err != nil {
			writeAppError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (h *Handler) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r.Context())
		if user == nil || user.Role != domain.RoleAdmin {
			writeAppError(w, domain.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAppError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrSeatUnavailable),
		errors.Is(err, domain.ErrLimitExceeded),
		errors.Is(err, domain.ErrBookingNotCancelable),
		errors.Is(err, domain.ErrConflictState),
		errors.Is(err, domain.ErrEmailTaken):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func parseWindowQuery(r *http.Request) (time.Time, time.Time, error) {
	startAt, err := time.Parse(time.RFC3339, r.URL.Query().Get("start_at"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endAt, err := time.Parse(time.RFC3339, r.URL.Query().Get("end_at"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startAt, endAt, nil
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func currentUser(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey).(*domain.User)
	return user
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
