-- Add photo URL columns for attendance (clock-in and clock-out)

ALTER TABLE attendance
  ADD COLUMN clock_in_photo_url VARCHAR(255) NULL AFTER clock_in_status,
  ADD COLUMN clock_out_photo_url VARCHAR(255) NULL AFTER clock_out_longitude;

