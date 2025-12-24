-- Per-event attendance spot tracking

ALTER TABLE attendance
  ADD COLUMN clock_in_spot_id BIGINT NULL AFTER attendance_spot_id,
  ADD COLUMN clock_out_spot_id BIGINT NULL AFTER clock_in_spot_id;

ALTER TABLE attendance
  ADD CONSTRAINT fk_attendance_clock_in_spot
    FOREIGN KEY (clock_in_spot_id) REFERENCES attendance_spots(id),
  ADD CONSTRAINT fk_attendance_clock_out_spot
    FOREIGN KEY (clock_out_spot_id) REFERENCES attendance_spots(id);

ALTER TABLE daily_activity_photos
  ADD COLUMN attendance_spot_id BIGINT NULL AFTER attendance_id;

ALTER TABLE daily_activity_photos
  ADD CONSTRAINT fk_dap_spot
    FOREIGN KEY (attendance_spot_id) REFERENCES attendance_spots(id);

-- Backfill per-event spot columns from existing attendance_spot_id

UPDATE attendance
SET clock_in_spot_id = attendance_spot_id
WHERE clock_in_spot_id IS NULL
  AND attendance_spot_id IS NOT NULL;

UPDATE attendance
SET clock_out_spot_id = attendance_spot_id
WHERE clock_out_spot_id IS NULL
  AND clock_out_time IS NOT NULL
  AND attendance_spot_id IS NOT NULL;

UPDATE daily_activity_photos dap
INNER JOIN attendance a ON a.id = dap.attendance_id
SET dap.attendance_spot_id = a.attendance_spot_id
WHERE dap.attendance_spot_id IS NULL
  AND a.attendance_spot_id IS NOT NULL;

