CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS seats (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    zone TEXT NOT NULL,
    type TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    booking_limit INT NOT NULL CHECK (booking_limit > 0 AND booking_limit <= 100)
);

INSERT INTO settings (id, booking_limit)
VALUES (1, 3)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS bookings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seat_id BIGINT NOT NULL REFERENCES seats(id) ON DELETE RESTRICT,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('confirmed', 'canceled', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (start_at < end_at)
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_status_end
    ON bookings (user_id, status, end_at);

CREATE INDEX IF NOT EXISTS idx_bookings_seat_time
    ON bookings (seat_id, start_at, end_at);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'bookings_no_confirmed_overlap'
    ) THEN
        ALTER TABLE bookings
            ADD CONSTRAINT bookings_no_confirmed_overlap
            EXCLUDE USING gist (
                seat_id WITH =,
                tstzrange(start_at, end_at, '[)') WITH &&
            )
            WHERE (status = 'confirmed');
    END IF;
END
$$;

INSERT INTO users (name, email, password_hash, role)
VALUES ('Admin', 'admin@mai.ru', '$2a$10$YKcXtoXzSrvSuApXIcLXGeTeuqfywM24JlY4YRJCfnCm6YFivgmgW', 'admin')
ON CONFLICT (email) DO NOTHING;

INSERT INTO seats (name, zone, type, active)
VALUES
    ('A-01', 'A', 'desk', TRUE),
    ('A-02', 'A', 'desk', TRUE),
    ('B-01', 'B', 'meeting', TRUE)
ON CONFLICT DO NOTHING;
