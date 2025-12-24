package datasources

import (
	"os"

	"github.com/jmoiron/sqlx"
	zero "github.com/rs/zerolog/log"
)

// Prepare prepare sql statements or exit api if fails or error
func Prepare(db *sqlx.DB, query string) *sqlx.Stmt {
	s, err := db.Preparex(query)
	if err != nil {
		zero.Error().Stack().
			Str("Context", "Preparing sql statement").
			Str("Query", query).
			Err(err).Msg("")

		os.Exit(4)
	}
	return s
}

// PrepareNamed prepare sql statements with named bindvars or exit api if fails or error
func PrepareNamed(db *sqlx.DB, query string) *sqlx.NamedStmt {
	s, err := db.PrepareNamed(query)
	if err != nil {
		zero.Error().Stack().
			Str("Context", "Preparing sql named statement").
			Str("Query", query).
			Err(err).Msg("")

		os.Exit(4)
	}
	return s
}
