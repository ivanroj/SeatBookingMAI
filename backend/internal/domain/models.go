package domain

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type BookingStatus string

const (
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCanceled  BookingStatus = "canceled"
	BookingStatusCompleted BookingStatus = "completed"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	DeviceID     string    `json:"device_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Coworking struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Capacity  int       `json:"capacity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Seat struct {
	ID          int64     `json:"id"`
	CoworkingID int64     `json:"coworking_id"`
	Name        string    `json:"name"`
	Zone        string    `json:"zone"`
	Type        string    `json:"type"`
	Label       string    `json:"label,omitempty"`
	GridX       int       `json:"grid_x"`
	GridY       int       `json:"grid_y"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Booking struct {
	ID          int64         `json:"id"`
	UserID      int64         `json:"user_id"`
	SeatID      int64         `json:"seat_id"`
	StartAt     time.Time     `json:"start_at"`
	EndAt       time.Time     `json:"end_at"`
	Status      BookingStatus `json:"status"`
	DisplayName string        `json:"display_name,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type Settings struct {
	BookingLimit int `json:"booking_limit"`
}
