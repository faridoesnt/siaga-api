package entities

import "time"

type User struct {
	ID            int64      `db:"id" json:"id"`
	Name          string     `db:"name" json:"name"`
	Email         string     `db:"email" json:"email"`
	PasswordHash  string     `db:"password_hash" json:"-"`
	Role          string     `db:"role" json:"role"`
	WorkStartDate *time.Time `db:"work_start_date" json:"work_start_date,omitempty"`
	Active        bool       `db:"active" json:"active"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

