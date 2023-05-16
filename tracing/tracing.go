package tracing

import (
	"fmt"
	"net/http"

	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

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

func Middleware(tp trace.TracerProvider, mp metric.MeterProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/-/health" {
				next.ServeHTTP(w, r)
				return
			}
			opts := []otelhttp.Option{
				otelhttp.WithTracerProvider(tp),
				otelhttp.WithMeterProvider(mp),
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
