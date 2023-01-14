package authz

import (
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/otel/trace"
)

type MiddlewareOption interface {
	middlewareOption()
}

type AuthenticateOption interface {
	autheticateOption()
}

type optTracerProvider struct {
	tp trace.TracerProvider
}

var _ MiddlewareOption = &optTracerProvider{}

func (*optTracerProvider) middlewareOption() {}

func WithTracerProvider(tp trace.TracerProvider) MiddlewareOption {
	return &optTracerProvider{tp: tp}
}

type optVerifyOptions struct {
	verifyOptions []jws.VerifyOption
}

var _ interface {
	MiddlewareOption
	AuthenticateOption
} = &optVerifyOptions{}

func (*optVerifyOptions) middlewareOption() {}

func (*optVerifyOptions) autheticateOption() {}

func WithVerifyOptions(opts ...jws.VerifyOption) MiddlewareOption {
	return &optVerifyOptions{verifyOptions: opts}
}

type optValidateOptions struct {
	validateOptions []jwt.ValidateOption
}

var _ interface {
	MiddlewareOption
	AuthenticateOption
} = &optValidateOptions{}

func (*optValidateOptions) middlewareOption() {}

func (*optValidateOptions) autheticateOption() {}

func WithValidateOptions(opts ...jwt.ValidateOption) MiddlewareOption {
	return &optValidateOptions{validateOptions: opts}
}

type optErrorHandler struct {
	handler ErrorHandlerFunc
}

var _ interface {
	MiddlewareOption
	AuthenticateOption
} = &optErrorHandler{}

func (*optErrorHandler) middlewareOption() {}

func (*optErrorHandler) autheticateOption() {}

func WithErrorHandler(handler ErrorHandlerFunc) MiddlewareOption {
	return &optErrorHandler{handler: handler}
}

type optTokenExtractor struct {
	extractor TokenExtractor
}

var _ interface {
	MiddlewareOption
	AuthenticateOption
} = &optTokenExtractor{}

func (*optTokenExtractor) middlewareOption() {}

func (*optTokenExtractor) autheticateOption() {}

func WithTokenExtractor(extractor TokenExtractor) MiddlewareOption {
	return &optTokenExtractor{extractor: extractor}
}
