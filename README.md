# SeatBookingMAI

Coworking seat-booking prototype:

- **`backend/`** — Go REST API (chi + pgx) with the booking domain logic and tests.
- **`frontend/`** — Telegram Mini-App style web client (static HTML/CSS/JS served by nginx).
- **`docker-compose.yml`** — full stack (PostgreSQL + backend + frontend) with healthchecks.
- **`Makefile`** — common dev commands.
- **`.github/workflows/ci.yml`** — gofmt + vet + test + docker build on each PR.

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
| POST   | `/api/auth/register`  | Register a new user |
| POST   | `/api/auth/login`     | Exchange credentials for a session token |

### Authenticated

| Method | Path                          | Description |
|--------|-------------------------------|-------------|
| GET    | `/api/auth/me`                | Current user |
| GET    | `/api/seats/available`        | Seats free in `start_at..end_at` (RFC3339) |
| POST   | `/api/bookings`               | Create a booking (`seat_id`, `start_at`, `end_at`) |
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
  migrations/001_init.sql # Schema, EXCLUDE constraint, seed admin/seats
frontend/
  index.html app.js styles.css nginx.conf Dockerfile
docker-compose.yml
Makefile
```
