-- Satpam profiles table: stores personal data for SATPAM users.

CREATE TABLE satpam_profiles (
    user_id BIGINT NOT NULL,
    jabatan VARCHAR(100) NOT NULL,
    jenis_kelamin ENUM('L','P') NOT NULL,
    tanggal_lahir DATE NULL,
    tempat_lahir VARCHAR(191) NULL,
    no_ktp VARCHAR(50) NULL,
    alamat TEXT NOT NULL,
    no_telepon VARCHAR(50) NOT NULL,
    agama VARCHAR(50) NULL,
    status_pernikahan VARCHAR(50) NULL,
    kebangsaan VARCHAR(50) NULL,
    work_start_date DATE NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id),
    KEY idx_satpam_profiles_work_start_date (work_start_date),
    CONSTRAINT fk_satpam_profiles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Backfill existing SATPAM users with basic profiles.
INSERT INTO satpam_profiles (
    user_id, jabatan, jenis_kelamin, tanggal_lahir, tempat_lahir, no_ktp,
    alamat, no_telepon, agama, status_pernikahan, kebangsaan, work_start_date
)
SELECT
    u.id,
    'Satpam' AS jabatan,
    'L' AS jenis_kelamin,
    NULL AS tanggal_lahir,
    NULL AS tempat_lahir,
    NULL AS no_ktp,
    '-' AS alamat,
    '-' AS no_telepon,
    NULL AS agama,
    NULL AS status_pernikahan,
    NULL AS kebangsaan,
    COALESCE(u.work_start_date, CURRENT_DATE)
FROM users u
WHERE u.role = 'SATPAM'
  AND NOT EXISTS (
    SELECT 1 FROM satpam_profiles p WHERE p.user_id = u.id
  );
