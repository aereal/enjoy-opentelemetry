package cache

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	attrCacheType = attribute.Key("graph.cache.type")
	attrCacheHit  = attribute.Key("graph.cache.hit")
	attrCacheKey  = attribute.Key("graph.cache.key")
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

type TracedCache struct {
	c      graphql.Cache
	attrs  []attribute.KeyValue
	tracer trace.Tracer
}

var _ graphql.Cache = (*TracedCache)(nil)

func NewTracedCache(c graphql.Cache, opts ...Option) *TracedCache {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.tracerProvider == nil {
		cfg.tracerProvider = otel.GetTracerProvider()
	}
	return &TracedCache{
		c:      c,
		attrs:  []attribute.KeyValue{attrCacheType.String(fmt.Sprintf("%T", c))},
		tracer: cfg.tracerProvider.Tracer("graph/cache.TracedCache"),
	}
}

func (c *TracedCache) Get(ctx context.Context, key string) (val any, ok bool) {
	attrs := c.attrs[:]
	attrs = append(attrs, attrCacheKey.String(key))
	ctx, span := c.tracer.Start(ctx, "Get", trace.WithAttributes(attrs...))
	defer func() {
		span.SetAttributes(attrCacheHit.Bool(ok))
		span.End()
	}()
	return c.c.Get(ctx, key)
}

func (c *TracedCache) Add(ctx context.Context, key string, val any) {
	attrs := c.attrs[:]
	attrs = append(attrs, attrCacheKey.String(key))
	ctx, span := c.tracer.Start(ctx, "Add", trace.WithAttributes(attrs...))
	defer func() {
		span.End()
	}()
	c.c.Add(ctx, key, val)
}
