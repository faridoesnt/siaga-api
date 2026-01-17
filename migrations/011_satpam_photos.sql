-- Add photo fields for SATPAM profiles (profile photo + ID card photo).

ALTER TABLE satpam_profiles
  ADD COLUMN photo_url VARCHAR(255) NULL AFTER work_start_date,
  ADD COLUMN ktp_photo_url VARCHAR(255) NULL AFTER photo_url;

