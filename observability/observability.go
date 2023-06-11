package observability

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

type config struct {
	ctx                  context.Context
	traceProviderOptions []sdktrace.TracerProviderOption
	metricReader         metric.Reader
	resourceAttributes   []attribute.KeyValue
}

type Option func(*config) error

func WithDeploymentEnvironment(env string) Option {
	return func(c *config) error {
		c.resourceAttributes = append(c.resourceAttributes, semconv.DeploymentEnvironmentKey.String(env))
		return nil
	}
}

func WithResourceName(name string) Option {
	return func(c *config) error {
		c.resourceAttributes = append(c.resourceAttributes, semconv.ServiceNameKey.String(name))
		return nil
	}
}

func WithDebugExporter(out io.Writer) Option {
	return func(c *config) error {
		debugExporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
			stdouttrace.WithoutTimestamps(),
			stdouttrace.WithWriter(out),
		)
		if err != nil {
			return fmt.Errorf("stdouttrace.New: %w", err)
		}
		c.traceProviderOptions = append(c.traceProviderOptions, sdktrace.WithBatcher(debugExporter))
		return nil
	}
}

func WithRemoteExporter() Option {
	return func(c *config) error {
		traceExpoter, err := otlptracegrpc.New(c.ctx, otlptracegrpc.WithInsecure())
		if err != nil {
			return fmt.Errorf("otlptracegrpc.New: %w", err)
		}
		c.traceProviderOptions = append(c.traceProviderOptions, sdktrace.WithBatcher(traceExpoter))
		metricExporter, err := otlpmetricgrpc.New(c.ctx, otlpmetricgrpc.WithInsecure())
		if err != nil {
			return fmt.Errorf("otlpmetricgrpc.New: %w", err)
		}
		c.metricReader = metric.NewPeriodicReader(metricExporter, metric.WithInterval(time.Second*10))
		return nil
	}
}

type Aggregate struct {
	TracerProvider *sdktrace.TracerProvider
	MetricProvider *metric.MeterProvider
}

func Setup(ctx context.Context, opts ...Option) (*Aggregate, error) {
	cfg := &config{ctx: ctx}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, err
		}
	}
	cfg.traceProviderOptions = append(cfg.traceProviderOptions, sdktrace.WithSampler(sdktrace.AlwaysSample()))
	res, err := prepareResource(ctx, cfg.resourceAttributes...)
	if err != nil {
		return nil, fmt.Errorf("prepareResource: %w", err)
	}
	cfg.traceProviderOptions = append(cfg.traceProviderOptions, sdktrace.WithResource(res))
	if cfg.metricReader == nil {
		cfg.metricReader = metric.NewManualReader()
	}
	aggr := &Aggregate{
		TracerProvider: sdktrace.NewTracerProvider(cfg.traceProviderOptions...),
		MetricProvider: metric.NewMeterProvider(
			metric.WithResource(res),
			metric.WithReader(cfg.metricReader),
		),
	}
	return aggr, nil
}

func prepareResource(ctx context.Context, attrs ...attribute.KeyValue) (*resource.Resource, error) {
	res, err := resource.New(
		ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("resource.New: %w", err)
	}
	return res, nil
}
