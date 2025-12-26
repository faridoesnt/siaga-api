package entities

import "time"

// SatpamProfile stores personal data for SATPAM users only.
type SatpamProfile struct {
	UserID          int64      `db:"user_id" json:"user_id"`
	Jabatan         string     `db:"jabatan" json:"jabatan"`
	JenisKelamin    string     `db:"jenis_kelamin" json:"jenis_kelamin"`
	TanggalLahir    *time.Time `db:"tanggal_lahir" json:"tanggal_lahir,omitempty"`
	TempatLahir     *string    `db:"tempat_lahir" json:"tempat_lahir,omitempty"`
	NoKTP           *string    `db:"no_ktp" json:"no_ktp,omitempty"`
	Alamat          string     `db:"alamat" json:"alamat"`
	NoTelepon       string     `db:"no_telepon" json:"no_telepon"`
	Agama           *string    `db:"agama" json:"agama,omitempty"`
	StatusPernikahan *string   `db:"status_pernikahan" json:"status_pernikahan,omitempty"`
	Kebangsaan      *string    `db:"kebangsaan" json:"kebangsaan,omitempty"`
	WorkStartDate   time.Time  `db:"work_start_date" json:"work_start_date"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
}

// SatpamUpsertPayload is used by services when creating/updating satpam data from admin APIs.
type SatpamUpsertPayload struct {
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Active          bool       `json:"is_active"`
	Jabatan         string     `json:"jabatan"`
	JenisKelamin    string     `json:"jenis_kelamin"`
	TanggalLahir    *time.Time `json:"tanggal_lahir,omitempty"`
	TempatLahir     *string    `json:"tempat_lahir,omitempty"`
	NoKTP           *string    `json:"no_ktp,omitempty"`
	Alamat          string     `json:"alamat"`
	NoTelepon       string     `json:"no_telepon"`
	Agama           *string    `json:"agama,omitempty"`
	StatusPernikahan *string   `json:"status_pernikahan,omitempty"`
	Kebangsaan      *string    `json:"kebangsaan,omitempty"`
	WorkStartDate   time.Time  `json:"work_start_date"`
}

// SatpamWithProfile is used in admin APIs to return combined account + profile data.
type SatpamWithProfile struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Active   bool   `json:"is_active"`

	Jabatan         string     `json:"jabatan"`
	JenisKelamin    string     `json:"jenis_kelamin"`
	TanggalLahir    *time.Time `json:"tanggal_lahir,omitempty"`
	TempatLahir     *string    `json:"tempat_lahir,omitempty"`
	NoKTP           *string    `json:"no_ktp,omitempty"`
	Alamat          string     `json:"alamat"`
	NoTelepon       string     `json:"no_telepon"`
	Agama           *string    `json:"agama,omitempty"`
	StatusPernikahan *string   `json:"status_pernikahan,omitempty"`
	Kebangsaan      *string    `json:"kebangsaan,omitempty"`
	WorkStartDate   time.Time  `json:"work_start_date"`
}
