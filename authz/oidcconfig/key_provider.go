package oidcconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrEmptyKeyID        = errors.New("kid is empty")
	ErrEmptyIssuerDomain = errors.New("issuer domain is empty")
	ErrRequestFailed     = errors.New("request failed")

	attrKeyID        = attribute.Key("jwk.key_id")
	attrKeyAlgorithm = attribute.Key("jwk.algorithm")
)

const (
	defaultOIDCEndpoint = "/.well-known/openid-configuration"
)

type config struct {
	tracerProvider trace.TracerProvider
	httpClient     *http.Client
	issuerDomain   string
	oidcConfigPath string
}

type Option func(c *config)

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		c.tracerProvider = tp
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *config) {
		c.httpClient = client
	}
}

func WithIssuer(issuerDomain string) Option {
	return func(c *config) {
		c.issuerDomain = issuerDomain
	}
}

func WithOpenIDConfigurationEndpointPath(path string) Option {
	return func(c *config) {
		c.oidcConfigPath = path
	}
}

func NewKeyProvider(opts ...Option) (*KeyProvider, error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.issuerDomain == "" {
		return nil, ErrEmptyIssuerDomain
	}
	if cfg.tracerProvider == nil {
		fmt.Println("! tracer provider is nil")
		cfg.tracerProvider = otel.GetTracerProvider()
	}
	if cfg.httpClient == nil {
		cfg.httpClient = http.DefaultClient
	}
	if cfg.oidcConfigPath == "" {
		cfg.oidcConfigPath = defaultOIDCEndpoint
	}
	kp := &KeyProvider{
		tracer:         cfg.tracerProvider.Tracer("enjoy-opentelemetry/authz/openid"),
		httpClient:     cfg.httpClient,
		issuerDomain:   cfg.issuerDomain,
		oidcConfigPath: cfg.oidcConfigPath,
	}
	return kp, nil
}

type KeyProvider struct {
	tracer         trace.Tracer
	httpClient     *http.Client
	issuerDomain   string
	oidcConfigPath string
}

var _ interface {
	jws.KeyProvider
	jwk.Fetcher
} = &KeyProvider{}

func (kp *KeyProvider) fetchKeysURI(ctx context.Context) (u string, err error) {
	ctx, span := kp.tracer.Start(ctx, "fetchKeysURI")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		span.End()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s%s", kp.issuerDomain, kp.oidcConfigPath), nil)
	if err != nil {
		return "", err
	}
	resp, err := kp.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ErrRequestFailed
	}
	var payload struct {
		JwksURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.JwksURI, nil
}

func (kp *KeyProvider) FetchKeys(ctx context.Context, sink jws.KeySink, sig *jws.Signature, msg *jws.Message) (err error) {
	ctx, span := kp.tracer.Start(ctx, "FetchKeys")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		span.End()
	}()

	kid := sig.ProtectedHeaders().KeyID()
	if kid == "" {
		return ErrEmptyKeyID
	}
	span.SetAttributes(attrKeyID.String(kid))

	keysURI, err := kp.fetchKeysURI(ctx)
	if err != nil {
		return err
	}
	set, err := kp.Fetch(ctx, keysURI)
	if err != nil {
		return err
	}
	key, ok := set.LookupKeyID(kid)
	if !ok {
		span.RecordError(&KeyNotFoundError{kid: kid})
		return nil
	}
	algs, err := jws.AlgorithmsForKey(key)
	if err != nil {
		return err
	}
	hdrAlg := sig.ProtectedHeaders().Algorithm()
	span.SetAttributes(attrKeyAlgorithm.String(hdrAlg.String()))
	for _, alg := range algs {
		if hdrAlg != "" && hdrAlg != alg {
			continue
		}
		sink.Key(alg, key)
		break
	}
	return nil
}

func (kp *KeyProvider) Fetch(ctx context.Context, uri string, opts ...jwk.FetchOption) (s jwk.Set, err error) {
	ctx, span := kp.tracer.Start(ctx, "Fetch")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		span.End()
	}()
	if len(opts) > 0 {
		_, logger := log.FromContext(ctx)
		logger.Warn(fmt.Sprintf("%T does not respect jwk.FetchOption; so ignored them", kp))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	resp, err := kp.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	set, err := jwk.ParseReader(resp.Body)
	if err != nil {
		return nil, err
	}
	return set, nil
}

type KeyNotFoundError struct {
	kid string
}

var _ error = &KeyNotFoundError{}

func (e *KeyNotFoundError) Error() string {
	return fmt.Sprintf("key for %q not found", e.kid)
}
