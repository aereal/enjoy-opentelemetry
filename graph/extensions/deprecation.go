package extensions

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func NewDeprecationNoticer() *DeprecationNoticer {
	return &DeprecationNoticer{}
}

type DeprecationNoticer struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.FieldInterceptor
} = (*DeprecationNoticer)(nil)

func (DeprecationNoticer) ExtensionName() string {
	return extensionName
}

func (DeprecationNoticer) Validate(_ graphql.ExecutableSchema) error {
	return nil
}

func (dn *DeprecationNoticer) InterceptField(ctx context.Context, next graphql.Resolver) (any, error) {
	fc := graphql.GetFieldContext(ctx)
	dir := fc.Field.Definition.Directives.ForName(directiveDeprecated)
	if dir != nil {
		err := gqlerror.WrapPath(fc.Path(), ErrDeprecatedFieldRequested)
		err.Extensions = map[string]interface{}{}
		if reason := dir.Arguments.ForName(argReason); reason != nil {
			err.Extensions[argReason] = reason.Value.String()
		}
		graphql.AddError(ctx, err)
	}
	return next(ctx)
}

const (
	extensionName       = "DeprecationNoticer"
	directiveDeprecated = "deprecated"
	argReason           = "reason"
)

var ErrDeprecatedFieldRequested = errors.New("deprecated field requested")
