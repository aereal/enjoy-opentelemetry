//go:generate go run github.com/99designs/gqlgen generate

package resolvers

import (
	"errors"

	"github.com/jmoiron/sqlx"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

func New(dbx *sqlx.DB) (*Resolver, error) {
	if dbx == nil {
		return nil, errors.New("dbx is nil")
	}
	return &Resolver{
		dbx: dbx,
	}, nil
}

type Resolver struct {
	dbx *sqlx.DB
}
