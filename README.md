# SeatBookingMAI

Coworking seat-booking prototype:

- **`backend/`** — Go REST API (chi + pgx) with the booking domain logic and tests.
- **`frontend/`** — single-page web client (static HTML/CSS/JS served by nginx).
- **`docker-compose.yml`** — full stack (PostgreSQL + backend + frontend) with healthchecks.
- **`Makefile`** — common dev commands.
- **`.github/workflows/ci.yml`** — gofmt + vet + test + docker build on each PR.

## UI flow

The single landing page asks **«Кто вы?»** and routes the user to one of two
role-gated screens, mirroring the actors and use cases from the spec:

| Role          | Authentication              | Visible use cases                       |
|---------------|-----------------------------|------------------------------------------|
| Студент / сотрудник | None — anonymous, device-bound. The browser stores a UUID in `localStorage`; the backend creates a hidden user for that device on first call. The student types their **name** in the booking form. | UC-2 (book), UC-3 (cancel), UC-4 (own bookings). |
| Администратор | Email + password (UC-1).    | UC-5 (seats), UC-6 (any booking), UC-7 (limit), UC-8 (reports). |

The student screen never exposes admin controls; the admin screen never
exposes student controls. A *Сменить роль* button at the top returns to the
landing page and clears in-memory tokens (the device id is preserved so the
student's bookings stay reachable after switching back).

## Run with Docker Compose

```bash
docker compose up --build
# or
make compose-up
```

Services:

| Service  | URL                          |
|----------|------------------------------|
| Frontend | <http://localhost:3000>      |
| Backend  | <http://localhost:8080>      |
| Postgres | `localhost:5432`             |

The compose file uses healthchecks so `frontend` only starts after `backend`
is healthy and `backend` only starts after the database is ready. You can
override credentials via `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`.

## Seed admin

The migration `backend/migrations/001_init.sql` seeds an admin and three seats:

- Email: `admin@mai.ru`
- Password: `admin123`

Migration `002_device_session.sql` adds the columns required by the student
device-session flow (`users.device_id` unique, `bookings.display_name`).

## Backend tests

```bash
cd backend
go test ./...
# or
make test          # or: make cover, make vet, make fmt-check
```

The suite covers the application service (registration / auth / booking /
admin operations) and the HTTP handlers via `httptest`. The postgres
repository is exercised end-to-end via docker compose.

## API

All routes return JSON and use `Authorization: Bearer <token>` for
authenticated endpoints.

### Public

| Method | Path                  | Description |
|--------|-----------------------|-------------|
| GET    | `/healthz`            | Liveness check |
| POST   | `/api/auth/register`  | Register a new user (kept for completeness; the UI uses it only for the legacy path) |
| POST   | `/api/auth/login`     | Exchange credentials for a session token (admin flow) |
| POST   | `/api/auth/device`    | Anonymous student session: `{device_id}` → `{token}`. Creates a hidden user on first call, reuses it on subsequent calls. |

### Authenticated

| Method | Path                          | Description |
|--------|-------------------------------|-------------|
| GET    | `/api/auth/me`                | Current user |
| GET    | `/api/seats/available`        | Seats free in `start_at..end_at` (RFC3339) |
| POST   | `/api/bookings`               | Create a booking (`seat_id`, `start_at`, `end_at`, optional `display_name` set by the student per booking) |
| GET    | `/api/bookings/me`            | List the caller's bookings |
| DELETE | `/api/bookings/{id}`          | Cancel own booking (must be ≥ 1h before start) |

### Admin

| Method | Path                               | Description |
|--------|------------------------------------|-------------|
| POST   | `/api/admin/seats`                 | Create seat |
| PUT    | `/api/admin/seats/{id}`            | Update seat |
| DELETE | `/api/admin/seats/{id}`            | Delete seat (rejected if future bookings exist) |
| GET    | `/api/admin/bookings`              | List all bookings |
| PATCH  | `/api/admin/bookings/{id}`         | Force booking status (`confirmed`/`canceled`/`completed`) |
| PUT    | `/api/admin/settings/limit`        | Update active-bookings-per-user limit (1..100) |
| GET    | `/api/admin/reports?from=...&to=...` | Aggregate report by seat / status |

## Configuration

The backend reads two environment variables:

| Variable        | Default                                                                 |
|-----------------|-------------------------------------------------------------------------|
| `DATABASE_URL`  | `postgres://postgres:postgres@localhost:5432/seat_booking?sslmode=disable` |
| `APP_PORT`      | `8080`                                                                  |

## Project layout

```
backend/
  cmd/server/             # entrypoint with graceful shutdown + DB retry
  internal/domain/        # Models and sentinel errors
  internal/app/           # Business service + Repository interface
  internal/app/apptest/   # In-memory FakeRepo used by tests
  internal/httpapi/       # chi-based HTTP handlers, middleware, error mapping
  internal/repo/postgres/ # pgx-backed repository
  migrations/001_init.sql           # Schema, EXCLUDE constraint, seed admin/seats
  migrations/002_device_session.sql # Anonymous student device sessions + display_name
frontend/
  index.html app.js styles.css nginx.conf Dockerfile
docker-compose.yml
Makefile
```
