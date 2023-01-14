package authz

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ctxKey = struct{ name string }{"AuthenticatedToken"}
)

func AuthenticatedToken(ctx context.Context) jwt.Token {
	if tok, ok := ctx.Value(ctxKey).(jwt.Token); ok {
		return tok
	}
	return nil
}

type mwConfig struct {
	tracerProvider  trace.TracerProvider
	verifyOptions   []jws.VerifyOption
	validateOptions []jwt.ValidateOption
	errorHandler    ErrorHandlerFunc
	tokenExtractor  TokenExtractor
}

func New(opts ...MiddlewareOption) *Middleware {
	cfg := &mwConfig{}
	for _, o := range opts {
		switch o := o.(type) {
		case *optErrorHandler:
			cfg.errorHandler = o.handler
		case *optTokenExtractor:
			cfg.tokenExtractor = o.extractor
		case *optTracerProvider:
			cfg.tracerProvider = o.tp
		case *optValidateOptions:
			cfg.validateOptions = o.validateOptions
		case *optVerifyOptions:
			cfg.verifyOptions = o.verifyOptions
		}
	}
	if cfg.tracerProvider == nil {
		cfg.tracerProvider = otel.GetTracerProvider()
	}
	if cfg.errorHandler == nil {
		cfg.errorHandler = defaultErrorHandler
	}
	if cfg.tokenExtractor == nil {
		cfg.tokenExtractor = ExtractFromHeader("authorization")
	}
	mw := &Middleware{
		tracer:          cfg.tracerProvider.Tracer("enjoy-opentelemetry/authz"),
		verifyOptions:   cfg.verifyOptions,
		validateOptions: cfg.validateOptions,
		errorHandler:    cfg.errorHandler,
		tokenExtractor:  cfg.tokenExtractor,
	}
	return mw
}

type Middleware struct {
	tracer          trace.Tracer
	verifyOptions   []jws.VerifyOption
	validateOptions []jwt.ValidateOption
	errorHandler    ErrorHandlerFunc
	tokenExtractor  TokenExtractor
}

type authenticateConfig struct {
	verifyOptions   []jws.VerifyOption
	validateOptions []jwt.ValidateOption
	errorHandler    ErrorHandlerFunc
	tokenExtractor  TokenExtractor
}

func (mw *Middleware) Authenticate(next http.Handler, opts ...AuthenticateOption) http.Handler {
	cfg := &authenticateConfig{
		errorHandler:    mw.errorHandler,
		tokenExtractor:  mw.tokenExtractor,
		validateOptions: mw.validateOptions,
		verifyOptions:   mw.verifyOptions,
	}
	for _, o := range opts {
		switch o := o.(type) {
		case *optErrorHandler:
			cfg.errorHandler = o.handler
		case *optTokenExtractor:
			cfg.tokenExtractor = o.extractor
		case *optValidateOptions:
			cfg.validateOptions = o.validateOptions
		case *optVerifyOptions:
			cfg.verifyOptions = o.verifyOptions
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentCtx := r.Context()
		ctx, span := mw.tracer.Start(parentCtx, "Authenticate")
		token, err := parseToken(ctx, r, cfg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			cfg.errorHandler(w, http.StatusUnauthorized, err.Error())
			return
		}
		span.End()
		next.ServeHTTP(w, r.WithContext(context.WithValue(parentCtx, ctxKey, token)))
	})
}

func parseToken(ctx context.Context, r *http.Request, cfg *authenticateConfig) (token jwt.Token, err error) {
	encodedToken, err := cfg.tokenExtractor.ExtractToken(r)
	if err != nil {
		return nil, err
	}
	verifyOpts := cfg.verifyOptions[:]
	verifyOpts = append(verifyOpts, jws.WithContext(ctx))
	sig, err := jws.Verify([]byte(encodedToken), verifyOpts...)
	if err != nil {
		return nil, err
	}
	tok := jwt.New()
	if err := json.Unmarshal(sig, tok); err != nil {
		return nil, err
	}
	validateOpts := cfg.validateOptions[:]
	validateOpts = append(validateOpts, jwt.WithContext(ctx))
	if err := jwt.Validate(tok, validateOpts...); err != nil {
		return nil, err
	}
	return tok, nil
}

type ErrorHandlerFunc func(w http.ResponseWriter, status int, msg string)

func defaultErrorHandler(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: msg})
}
