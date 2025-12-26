-- Permissions and user_permissions for admin RBAC

CREATE TABLE permissions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    label VARCHAR(191) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_permissions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    permission_code VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_user_permission (user_id, permission_code),
    CONSTRAINT fk_user_permissions_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_user_permissions_permission FOREIGN KEY (permission_code) REFERENCES permissions(code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed all permission codes

INSERT INTO permissions (code, label) VALUES
  ('DASHBOARD_VIEW', 'Lihat dashboard attendance'),
  ('SATPAM_VIEW', 'Lihat data satpam'),
  ('SATPAM_MANAGE', 'Kelola data satpam'),
  ('ATTENDANCE_SPOT_VIEW', 'Lihat attendance spot'),
  ('ATTENDANCE_SPOT_MANAGE', 'Kelola attendance spot'),
  ('SHIFT_VIEW', 'Lihat shift'),
  ('SHIFT_MANAGE', 'Kelola shift'),
  ('SCHEDULING_VIEW', 'Lihat penjadwalan shift'),
  ('SCHEDULING_MANAGE', 'Kelola penjadwalan shift'),
  ('SPOT_ASSIGNMENT_VIEW', 'Lihat penugasan spot'),
  ('SPOT_ASSIGNMENT_MANAGE', 'Kelola penugasan spot'),
  ('SHIFT_SWAP_VIEW', 'Lihat riwayat tukar shift'),
  ('ATTENDANCE_MONITORING_VIEW', 'Lihat monitoring kehadiran'),
  ('ATTENDANCE_MONITORING_MANAGE', 'Kelola monitoring/override kehadiran'),
  ('ADMIN_VIEW', 'Lihat admin'),
  ('ADMIN_MANAGE', 'Kelola admin');

