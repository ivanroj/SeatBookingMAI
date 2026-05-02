-- Allow cascading seat deletes through to bookings.
--
-- The original schema used ON DELETE RESTRICT on bookings.seat_id, which made
-- it impossible to delete a coworking (and its seats) once any booking ever
-- existed. We now cancel future bookings explicitly in the service layer
-- (so admins still see what was affected in the journal) and rely on the
-- cascade to clean up any historical rows.

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS bookings_seat_id_fkey;

ALTER TABLE bookings
    ADD CONSTRAINT bookings_seat_id_fkey
        FOREIGN KEY (seat_id)
        REFERENCES seats(id)
        ON DELETE CASCADE;
