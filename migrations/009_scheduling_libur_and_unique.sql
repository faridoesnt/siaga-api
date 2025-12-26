-- Ensure 'Libur' shift exists
INSERT INTO shifts (name, start_time, end_time, late_tolerance_minute)
SELECT 'Libur', '00:00:00', '00:00:00', 0
WHERE NOT EXISTS (
  SELECT 1 FROM shifts WHERE name = 'Libur'
);

-- Ensure unique key on user_shifts(user_id, shift_date)
ALTER TABLE user_shifts
  ADD UNIQUE KEY uq_user_shift_user_date (user_id, shift_date);

