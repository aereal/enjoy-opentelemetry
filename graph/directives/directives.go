package directives

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/enjoy-opentelemetry/authz"
	"github.com/aereal/enjoy-opentelemetry/graph"
	"github.com/aereal/enjoy-opentelemetry/graph/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type config struct {
	tracerProvider trace.TracerProvider
}

type Option func(c *config)

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		c.tracerProvider = tp
	}
}

var (
	ErrUnauthenticated        = errors.New("unauthenticated")
	ErrInsufficientPermission = errors.New("insufficient permission")

	keyRequiredPermission = attribute.Key("authz.required_permission")
	keyAllowedPermission  = attribute.Key("authz.allowed_permission")
)

func New(opts ...Option) graph.DirectiveRoot {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.tracerProvider == nil {
		cfg.tracerProvider = otel.GetTracerProvider()
	}

	tracer := cfg.tracerProvider.Tracer("enjoy-opentelemetry/graph/directives")
	root := graph.DirectiveRoot{}
	root.Authenticate = func(parentCtx context.Context, obj any, next graphql.Resolver, scopes []models.Scope) (res any, err error) {
		ctx, span := tracer.Start(parentCtx, "Authenticate")
		defer func() {
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
			}
			span.End()
		}()
		token := authz.AuthenticatedToken(ctx)
		if token == nil {
			return nil, ErrUnauthenticated
		}
		requiredPermissions := models.NewPermission(scopes...)
		allowedPermissions := models.ParsePermissionClaim(token.Get("permissions"))
		span.SetAttributes(
			keyRequiredPermission.StringSlice(requiredPermissions.Strings()),
			keyAllowedPermission.StringSlice(allowedPermissions.Strings()),
		)
		if !allowedPermissions.IsSuperSetOf(requiredPermissions) {
			return nil, ErrInsufficientPermission
		}
		return next(parentCtx)
	}
	return root
}
