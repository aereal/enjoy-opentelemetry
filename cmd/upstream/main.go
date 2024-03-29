package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/observability"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aereal/enjoy-opentelemetry/upstream"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

var (
	shutdownTimeout = time.Second * 5

	upstreamPort     string
	downstreamOrigin string
	deploymentEnv    string
	serviceName      string
	debug            bool
	envDebug         = os.Getenv("DEBUG")
)

func init() {
	flag.StringVar(&upstreamPort, "upstream-port", os.Getenv("PORT"), "upstream server port")
	flag.StringVar(&downstreamOrigin, "downstream-origin", os.Getenv("DOWNSTREAM_ORIGIN"), "downstream origin")
	flag.StringVar(&deploymentEnv, "env", os.Getenv("APP_ENV"), "deployment environment")
	flag.StringVar(&serviceName, "service", os.Getenv("APP_SERVICE_NAME"), "service name")
	flag.BoolVar(&debug, "debug", envDebug != "", "debug mode")
}

func run() error {
	flag.Parse()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(xray.Propagator{}))
	setupCtx, logger := log.FromContext(context.Background())
	upstreamAggr, cleanupUpstream, err := setupObservability(setupCtx, "upstream")
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		defer cleanupUpstream(ctx)
	}()
	rt := otelhttp.NewTransport(
		&tracing.ResourceOverriderRoundTripper{Base: http.DefaultTransport},
		otelhttp.WithTracerProvider(upstreamAggr.TracerProvider),
	)
	logger.Info(
		"start server",
		zap.String("component", "upstream"),
		zap.String("env", deploymentEnv),
		zap.String("service", serviceName),
		zap.String("port", upstreamPort),
		zap.Bool("debug", debug))
	upstreamApp, err := upstream.New(
		upstreamAggr.TracerProvider,
		upstreamAggr.MetricProvider,
		&http.Client{Transport: rt},
		downstreamOrigin,
	)
	if err != nil {
		return fmt.Errorf("upstream.New: %w", err)
	}
	upstreamSrv := &http.Server{
		Addr:    fmt.Sprintf(":%s", upstreamPort),
		Handler: upstreamApp.Handler(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	go graceful(ctx, upstreamSrv)
	logger.Info("start listening", zap.String("addr", upstreamSrv.Addr))
	if err := upstreamSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "! %+v\n", err)
		exitCode := 1
		if err, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = err.ExitCode()
		}
		os.Exit(exitCode)
	}
}

func graceful(ctx context.Context, srv *http.Server) {
	ctx, logger := log.FromContext(ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	logger.Info("received signal", zap.Stringer("signal", sig))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("failed to gracefully shutdown server", zap.Error(err))
	}
	logger.Info("shutting down server")
}

var noop = func(context.Context) {}

func setupObservability(ctx context.Context, component string) (*observability.Aggregate, func(context.Context), error) {
	opts := []observability.Option{
		observability.WithRemoteExporter(),
		observability.WithDeploymentEnvironment(deploymentEnv),
		observability.WithResourceName(fmt.Sprintf("%s-%s", serviceName, component)),
	}
	if debug {
		opts = append(opts, observability.WithDebugExporter(os.Stderr))
	}
	aggr, err := observability.Setup(ctx, opts...)
	if err != nil {
		return nil, noop, fmt.Errorf("%s: tracing.Setup: %w", component, err)
	}
	cleanup := func(ctx context.Context) {
		_, logger := log.FromContext(ctx)
		if err := aggr.TracerProvider.Shutdown(ctx); err != nil {
			logger.Error("failed to cleanup otel trace provider", zap.String("component", component), zap.Error(err))
		}
		if err := aggr.MetricProvider.Shutdown(ctx); err != nil {
			logger.Error("failed to cleanup otel metric provider", zap.String("component", component), zap.Error(err))
		}
	}
	return aggr, cleanup, nil
}
