package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aereal/enjoy-opentelemetry/downstream"
	"github.com/aereal/enjoy-opentelemetry/graceful"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	shutdownTimeout  = time.Second * 5
	httpClient       = otelhttp.DefaultClient
	attrResourceName = attribute.Key("resource.name")
)

func withTrace() func(http.Handler) http.Handler {
	formatter := otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
		if routeKey := httptreemux.ContextRoute(r.Context()); routeKey != "" {
			return routeKey
		}
		return operation
	})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			opts := []otelhttp.Option{
				formatter,
			}
			if routeKey := httptreemux.ContextRoute(r.Context()); routeKey != "" {
				opts = append(opts, otelhttp.WithSpanOptions(trace.WithAttributes(attrResourceName.String(routeKey))))
			}
			h := otelhttp.NewHandler(next, "enjoy-opentelemetry", opts...)
			h.ServeHTTP(w, r)
		})
	}
}

func run() error {
	setupCtx := context.Background()
	tp, err := tracing.Setup(
		setupCtx,
		tracing.WithDebugExporter(os.Stderr),
		tracing.WithHTTPExporter(),
		tracing.WithDeploymentEnvironment("local"),
		tracing.WithResourceName("enjoy-opentelemetry"),
	)
	if err != nil {
		return fmt.Errorf("tracing.Setup: %w", err)
	}
	otel.SetTracerProvider(tp)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("failed to cleanup otel trace provider: %s", err)
		}
	}()
	cfg, err := config.LoadDefaultConfig(
		setupCtx,
		config.WithHTTPClient(httpClient),
		config.WithEndpointDiscovery(aws.EndpointDiscoveryDisabled),
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled),
		config.WithEC2RoleCredentialOptions(nil),
	)
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	otelaws.AppendMiddlewares(&cfg.APIOptions)
	stsClient := sts.NewFromConfig(cfg)
	downstreamApp, err := downstream.New(stsClient)
	if err != nil {
		return fmt.Errorf("downstream.New: %w", err)
	}
	mux := httptreemux.NewContextMux()
	mux.UseHandler(withTrace())
	group := mux.NewGroup("/downstream")
	downstreamApp.DefineRoutes(group)
	downstreamSrv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := graceful.StartServer(ctx, downstreamSrv); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("! %+v", err)
		exitCode := 1
		if err, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = err.ExitCode()
		}
		os.Exit(exitCode)
	}
}
