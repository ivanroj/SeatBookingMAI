-- Coworkings + seat grid layout.
--
-- A coworking is a top-level container for a set of seats. Each seat lives at
-- coordinates (grid_x, grid_y) inside its coworking so the UI can render an
-- interactive map.

CREATE TABLE IF NOT EXISTS coworkings (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    capacity    INT  NOT NULL CHECK (capacity > 0 AND capacity <= 1000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed a default coworking so existing rows can be backfilled non-destructively.
INSERT INTO coworkings (name, capacity)
VALUES ('Главный коворкинг', 50)
ON CONFLICT (name) DO NOTHING;

ALTER TABLE seats
    ADD COLUMN IF NOT EXISTS coworking_id BIGINT REFERENCES coworkings(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS grid_x       INT,
    ADD COLUMN IF NOT EXISTS grid_y       INT,
    ADD COLUMN IF NOT EXISTS label        TEXT;

-- Backfill: any seat without a coworking goes to the default one and gets a
-- placement on a single row so the admin map renders something sensible.
WITH defaults AS (
    SELECT id FROM coworkings WHERE name = 'Главный коворкинг' LIMIT 1
), to_fill AS (
    SELECT
        s.id,
        ROW_NUMBER() OVER (ORDER BY s.id) - 1 AS rn
    FROM seats s
    WHERE s.coworking_id IS NULL
)
UPDATE seats s
SET coworking_id = (SELECT id FROM defaults),
    grid_x       = COALESCE(s.grid_x, to_fill.rn % 8),
    grid_y       = COALESCE(s.grid_y, to_fill.rn / 8),
    label        = COALESCE(s.label, s.name)
FROM to_fill
WHERE s.id = to_fill.id;

ALTER TABLE seats
    ALTER COLUMN coworking_id SET NOT NULL,
    ALTER COLUMN grid_x       SET NOT NULL,
    ALTER COLUMN grid_y       SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_seats_coworking_position
    ON seats (coworking_id, grid_x, grid_y);

CREATE UNIQUE INDEX IF NOT EXISTS idx_seats_coworking_name
    ON seats (coworking_id, name);

CREATE INDEX IF NOT EXISTS idx_seats_coworking
    ON seats (coworking_id);
