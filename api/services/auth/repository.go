package auth

import (
	"context"
	"database/sql"

	"siaga-api/api/contracts"
	"siaga-api/api/datasources"
	"siaga-api/api/entities"

	"github.com/jmoiron/sqlx"
)

type Repository struct {
	app  *contracts.App
	stmt Statement
}

type Statement struct {
	example             *sqlx.Stmt
}

func initRepository(app *contracts.App) contracts.AuthRepository {
	stmts := Statement{
		example:             datasources.Prepare(app.Ds.ReaderDB, example),
	}

	r := Repository{
		app:  app,
		stmt: stmts,
	}

	return &r
}

// FindByEmail returns user by email or nil if not found.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	var user entities.User
	err := r.app.Ds.ReaderDB.GetContext(ctx, &user, `
		SELECT id, name, email, password_hash, role, work_start_date, active, created_at, updated_at
		FROM users
		WHERE email = ?
		LIMIT 1
	`, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
