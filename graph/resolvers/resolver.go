//go:generate go run github.com/99designs/gqlgen generate

package resolvers

import (
	"errors"

	"github.com/aereal/enjoy-opentelemetry/domain"
	"github.com/jmoiron/sqlx"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

func New(liverGroupRepository *domain.LiverGroupRepository, dbx *sqlx.DB) (*Resolver, error) {
	if liverGroupRepository == nil {
		return nil, errors.New("liverGroupRepository is nil")
	}
	if dbx == nil {
		return nil, errors.New("dbx is nil")
	}
	return &Resolver{
		liverGroupRepository: liverGroupRepository,
		dbx:                  dbx,
	}, nil
}

type Resolver struct {
	liverGroupRepository *domain.LiverGroupRepository
	dbx                  *sqlx.DB
}
