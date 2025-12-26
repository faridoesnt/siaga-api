package entities

type Permission struct {
	Code  string `db:"code" json:"code"`
	Label string `db:"label" json:"label"`
}

