-- Initial schema for siaga-api

CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(191) NOT NULL UNIQUE,
    password_hash VARCHAR(191) NOT NULL,
    role ENUM('SATPAM', 'ADMIN') NOT NULL,
    work_start_date DATE NULL,
    active TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE attendance_spots (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    latitude DOUBLE NOT NULL,
    longitude DOUBLE NOT NULL,
    radius_meters INT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_attendance_spots (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    attendance_spot_id BIGINT NOT NULL,
    active_from DATE NOT NULL,
    active_until DATE NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_uas_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_uas_spot FOREIGN KEY (attendance_spot_id) REFERENCES attendance_spots(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE shifts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    late_tolerance_minute INT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_shifts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    shift_id BIGINT NOT NULL,
    shift_date DATE NOT NULL,
    is_swapped TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_user_shift_date (user_id, shift_date),
    CONSTRAINT fk_user_shifts_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_user_shifts_shift FOREIGN KEY (shift_id) REFERENCES shifts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE attendance (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    shift_id BIGINT NOT NULL,
    attendance_spot_id BIGINT NULL,
    attendance_date DATE NOT NULL,
    clock_in_time DATETIME NULL,
    clock_in_latitude DOUBLE NULL,
    clock_in_longitude DOUBLE NULL,
    clock_in_status ENUM('ON_TIME', 'LATE', 'TOO_LATE') NULL,
    face_verified TINYINT(1) NOT NULL DEFAULT 0,
    face_match_score DOUBLE NULL,
    clock_out_time DATETIME NULL,
    clock_out_latitude DOUBLE NULL,
    clock_out_longitude DOUBLE NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_attendance_user_date (user_id, attendance_date),
    CONSTRAINT fk_attendance_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_attendance_shift FOREIGN KEY (shift_id) REFERENCES shifts(id),
    CONSTRAINT fk_attendance_spot FOREIGN KEY (attendance_spot_id) REFERENCES attendance_spots(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE face_embeddings (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    embedding TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_face_embeddings_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE daily_activity_photos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    attendance_id BIGINT NOT NULL,
    photo_url VARCHAR(255) NOT NULL,
    note TEXT NULL,
    taken_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_dap_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_dap_attendance FOREIGN KEY (attendance_id) REFERENCES attendance(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE shift_swap_requests (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    requester_user_id BIGINT NOT NULL,
    target_user_id BIGINT NOT NULL,
    shift_date DATE NOT NULL,
    requester_user_shift_id BIGINT NOT NULL,
    target_user_shift_id BIGINT NOT NULL,
    status ENUM('PENDING', 'APPROVED', 'REJECTED') NOT NULL DEFAULT 'PENDING',
    reason TEXT,
    note TEXT,
    decided_by BIGINT NULL,
    decided_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_ssr_requester FOREIGN KEY (requester_user_id) REFERENCES users(id),
    CONSTRAINT fk_ssr_target FOREIGN KEY (target_user_id) REFERENCES users(id),
    CONSTRAINT fk_ssr_requester_shift FOREIGN KEY (requester_user_shift_id) REFERENCES user_shifts(id),
    CONSTRAINT fk_ssr_target_shift FOREIGN KEY (target_user_shift_id) REFERENCES user_shifts(id),
    CONSTRAINT fk_ssr_decided_by FOREIGN KEY (decided_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
