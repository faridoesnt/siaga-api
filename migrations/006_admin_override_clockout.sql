-- Admin override metadata for attendance clock-out

ALTER TABLE attendance
  ADD COLUMN override_by_admin_id BIGINT NULL AFTER clock_out_photo_url,
  ADD COLUMN override_at DATETIME NULL AFTER override_by_admin_id,
  ADD COLUMN override_reason TEXT NULL AFTER override_at;

ALTER TABLE attendance
  ADD CONSTRAINT fk_attendance_override_admin
    FOREIGN KEY (override_by_admin_id) REFERENCES users(id);

