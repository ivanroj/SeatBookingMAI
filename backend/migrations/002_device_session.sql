-- Adds anonymous device-bound sessions for the student flow:
-- a student does not register or log in; the device gets a UUID stored
-- locally and that maps to a hidden user with role=user.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS device_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_device_id
    ON users (device_id)
    WHERE device_id IS NOT NULL;

-- Per-booking display name typed by the student at booking time.
-- Optional for backwards compatibility with rows created before this migration
-- and with the (still supported) authenticated-user code path.
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS display_name TEXT;
