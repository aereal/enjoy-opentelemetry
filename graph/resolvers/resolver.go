//go:generate go run github.com/99designs/gqlgen generate

package resolvers

import (
	"errors"

	"github.com/aereal/enjoy-opentelemetry/domain"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

func New(liverRepository *domain.LiverRepository) (*Resolver, error) {
	if liverRepository == nil {
		return nil, errors.New("domain.LiverRepository is nil")
	}
	return &Resolver{
		liverRepository: liverRepository,
	}, nil
}

type Resolver struct {
	liverRepository *domain.LiverRepository
}
