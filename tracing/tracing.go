package tracing

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

type config struct {
	ctx                  context.Context
	traceProviderOptions []sdktrace.TracerProviderOption
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

func WithHTTPExporter() Option {
	return func(c *config) error {
		httpExporter, err := otlptracehttp.New(c.ctx, otlptracehttp.WithInsecure())
		if err != nil {
			return fmt.Errorf("otlptracehttp.New: %w", err)
		}
		c.traceProviderOptions = append(c.traceProviderOptions, sdktrace.WithBatcher(httpExporter))
		return nil
	}
}

func Setup(ctx context.Context, opts ...Option) (*sdktrace.TracerProvider, error) {
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
	tp := sdktrace.NewTracerProvider(cfg.traceProviderOptions...)
	return tp, nil
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

func ReportError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}

var (
	attrResourceName = attribute.Key("resource.name")
)

func Middleware(tp trace.TracerProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/-/health" {
				next.ServeHTTP(w, r)
				return
			}
			opts := []otelhttp.Option{
				otelhttp.WithTracerProvider(tp),
			}
			opName := "unknown"
			if routeData := httptreemux.ContextData(r.Context()); routeData != nil {
				routeKey := routeData.Route()
				opName = routeKey
				attrs := []attribute.KeyValue{
					attrResourceName.String(routeKey),
					semconv.HTTPRouteKey.String(routeKey),
				}
				for k, v := range routeData.Params() {
					a := attribute.String(fmt.Sprintf("http.route_params.%s", k), v)
					attrs = append(attrs, a)
				}
				opts = append(opts, otelhttp.WithSpanOptions(trace.WithAttributes(attrs...)))
			}
			h := otelhttp.NewHandler(next, opName, opts...)
			h.ServeHTTP(w, r)
		})
	}
}

type ResourceOverriderRoundTripper struct {
	Base http.RoundTripper
}

var _ http.RoundTripper = (*ResourceOverriderRoundTripper)(nil)

func (rt *ResourceOverriderRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(r.Context())

	userinfo := r.URL.User
	r.URL.User = nil
	defer func() {
		r.URL.User = userinfo
	}()

	span.SetAttributes(attrResourceName.String(r.URL.String()))
	return rt.Base.RoundTrip(r)
}

var (
	sqsEventAttributes = []attribute.KeyValue{
		semconv.MessagingSystemKey.String("AmazonSQS"),
		semconv.MessagingOperationProcess,
		semconv.FaaSTriggerPubsub,
		attribute.String("messaging.source.kind", "queue"),
	}
	messagingSourceNameKey = attribute.Key("messaging.source.name")
	messagingMessageIDKey  = attribute.Key("messaging.message.id")
)

const (
	attrAWSTraceHeader = "AWSTraceHeader"
	headerXrayID       = "X-Amzn-Trace-Id"
)

func StartTraceFromSQSEvent(ctx context.Context, tracer trace.Tracer, queueName string) (context.Context, trace.Span) {
	attrs := sqsEventAttributes[:]
	attrs = append(attrs, messagingSourceNameKey.String(queueName))
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...),
	}
	return tracer.Start(ctx, fmt.Sprintf("%s process", queueName), opts...)
}

func StartTraceFromSQSMessage(ctx context.Context, tracer trace.Tracer, queueName string, msg events.SQSMessage) (context.Context, trace.Span) {
	attrs := sqsEventAttributes[:]
	attrs = append(attrs, messagingSourceNameKey.String(queueName), messagingMessageIDKey.String(msg.MessageId))
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...),
	}
	if traceID, ok := msg.Attributes[attrAWSTraceHeader]; ok {
		carrier := propagation.MapCarrier{}
		carrier.Set(headerXrayID, traceID)
		remoteCtx := xray.Propagator{}.Extract(ctx, carrier)
		link := trace.LinkFromContext(remoteCtx)
		opts = append(opts, trace.WithLinks(link))
	}
	return tracer.Start(ctx, fmt.Sprintf("%s process", queueName), opts...)
}
