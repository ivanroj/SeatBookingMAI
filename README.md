# SeatBookingMAI

Prototype of a coworking seat booking system:

- `backend/` - Go REST API with booking business rules and tests.
- `frontend/` - Telegram Mini App style web client.
- `docker-compose.yml` - PostgreSQL + backend + frontend.

## Run

```bash
docker compose up --build
```

Services:

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`
- DB: `localhost:5432`

## Seed admin

- Email: `admin@mai.ru`
- Password: `admin123`

## Backend tests

```bash
cd backend
go test ./...
```
