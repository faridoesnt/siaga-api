package entities

import "time"

type FaceEmbedding struct {
	ID        int64      `db:"id" json:"id"`
	UserID    int64      `db:"user_id" json:"user_id"`
	Embedding string     `db:"embedding" json:"embedding"`
	Model     *string    `db:"model" json:"model,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

type FaceEmbeddingSummary struct {
	UserID    int64      `json:"user_id"`
	Count     int        `json:"count"`
	Model     *string    `json:"model,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

