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

	"github.com/aereal/enjoy-opentelemetry/adapters/db"
	"github.com/aereal/enjoy-opentelemetry/authz"
	"github.com/aereal/enjoy-opentelemetry/authz/oidcconfig"
	"github.com/aereal/enjoy-opentelemetry/domain"
	"github.com/aereal/enjoy-opentelemetry/downstream"
	"github.com/aereal/enjoy-opentelemetry/graph/loaders"
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/observability"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

var (
	shutdownTimeout = time.Second * 5

	downstreamPort string
	deploymentEnv  string
	serviceName    string
	debug          bool
	envDebug       = os.Getenv("DEBUG")
)

func init() {
	flag.StringVar(&downstreamPort, "downstream-port", os.Getenv("PORT"), "downstream server port")
	flag.StringVar(&deploymentEnv, "env", os.Getenv("APP_ENV"), "deployment environment")
	flag.StringVar(&serviceName, "service", os.Getenv("APP_SERVICE_NAME"), "service name")
	flag.BoolVar(&debug, "debug", envDebug != "", "debug mode")
}

func run() error {
	flag.Parse()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(xray.Propagator{}))
	setupCtx, logger := log.FromContext(context.Background())
	downAggr, cleanupDownstream, err := setupObservability(setupCtx, "downstream")
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		defer cleanupDownstream(ctx)
	}()
	logger.Info(
		"start server",
		zap.String("component", "downstream"),
		zap.String("env", deploymentEnv),
		zap.String("service", serviceName),
		zap.String("port", downstreamPort),
		zap.Bool("debug", debug))
	dbx, err := db.New(os.Getenv("DSN"), db.WithTracerProvider(downAggr.TracerProvider), db.WithMetricProvider(downAggr.MetricProvider))
	if err != nil {
		return fmt.Errorf("db.New: %w", err)
	}
	newRepositoryOptions := []domain.NewRepositoryOption{
		domain.WithDB(dbx),
		domain.WithTracerProvider(downAggr.TracerProvider),
	}
	liverGroupRepository, err := domain.NewLiverGroupRepository(newRepositoryOptions...)
	if err != nil {
		return err
	}
	liverRepository, err := domain.NewLiverRepository(newRepositoryOptions...)
	if err != nil {
		return err
	}
	rootResolver, err := resolvers.New(liverRepository)
	if err != nil {
		return fmt.Errorf("resolvers.New: %w", err)
	}
	rt := otelhttp.NewTransport(
		&tracing.ResourceOverriderRoundTripper{Base: http.DefaultTransport},
		otelhttp.WithTracerProvider(downAggr.TracerProvider),
	)
	httpClient := &http.Client{Transport: rt}
	kp, err := oidcconfig.NewKeyProvider(
		oidcconfig.WithHTTPClient(httpClient),
		oidcconfig.WithIssuer(os.Getenv("AUTH0_ISSUER")),
		oidcconfig.WithTracerProvider(downAggr.TracerProvider),
	)
	if err != nil {
		return err
	}
	mw := authz.New(
		authz.WithTracerProvider(downAggr.TracerProvider),
		authz.WithTokenExtractor(authz.ExtractFromAuthorizationHeader()),
		authz.WithVerifyOptions(jws.WithKeyProvider(kp)),
		authz.WithValidateOptions(jwt.WithAudience(os.Getenv("AUTH0_AUDIENCE"))),
	)
	loaderAggregate, err := loaders.NewAggregate(liverGroupRepository, loaders.WithTracerProvider(downAggr.TracerProvider))
	if err != nil {
		return err
	}
	downstreamApp, err := downstream.New(downAggr.TracerProvider, downAggr.MetricProvider, rootResolver, mw, loaderAggregate)
	if err != nil {
		return fmt.Errorf("downstream.New: %w", err)
	}
	downstreamSrv := &http.Server{
		Addr:    fmt.Sprintf(":%s", downstreamPort),
		Handler: downstreamApp.Handler(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	go graceful(ctx, downstreamSrv)
	logger.Info("start listening", zap.String("addr", downstreamSrv.Addr))
	if err := downstreamSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
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
		observability.WithHTTPExporter(),
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
