package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	"github.com/aereal/enjoy-opentelemetry/upstream"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

var (
	shutdownTimeout = time.Second * 5

	upstreamPort   int
	downstreamPort int
	deploymentEnv  string
	serviceName    string
	debug          bool
)

func init() {
	flag.IntVar(&upstreamPort, "upstream-port", 8080, "upstream server port")
	flag.IntVar(&downstreamPort, "downstream-port", 8081, "downstream server port")
	flag.StringVar(&deploymentEnv, "env", "local", "deployment environment")
	flag.StringVar(&serviceName, "service", "enjoy-opentelemetry", "service name")
	flag.BoolVar(&debug, "debug", false, "debug mode")
}

func run() error {
	flag.Parse()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, xray.Propagator{}))
	setupCtx := context.Background()
	upstreamAggr, cleanupUpstream, err := setupObservability(setupCtx, "upstream")
	if err != nil {
		return err
	}
	downstreamAggr, cleanupDownstream, err := setupObservability(setupCtx, "downstream")
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		defer cleanupDownstream(ctx)
		defer cleanupUpstream(ctx)
	}()
	dbx, err := db.New(os.Getenv("DSN"), db.WithTracerProvider(downstreamAggr.TracerProvider), db.WithMetricProvider(downstreamAggr.MetricProvider))
	if err != nil {
		return fmt.Errorf("db.New: %w", err)
	}
	newRepositoryOptions := []domain.NewRepositoryOption{
		domain.WithDB(dbx),
		domain.WithTracerProvider(downstreamAggr.TracerProvider),
		domain.WithMetricProvider(downstreamAggr.MetricProvider),
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
	baseTransport := &tracing.ResourceOverriderRoundTripper{Base: http.DefaultTransport}
	upstreamHTTPClient := &http.Client{
		Transport: otelhttp.NewTransport(baseTransport, otelhttp.WithTracerProvider(upstreamAggr.TracerProvider)),
	}
	downstreamHTTPClient := &http.Client{
		Transport: otelhttp.NewTransport(baseTransport, otelhttp.WithTracerProvider(downstreamAggr.TracerProvider)),
	}
	kp, err := oidcconfig.NewKeyProvider(
		oidcconfig.WithHTTPClient(downstreamHTTPClient),
		oidcconfig.WithIssuer(os.Getenv("AUTH0_ISSUER")),
		oidcconfig.WithTracerProvider(downstreamAggr.TracerProvider),
	)
	if err != nil {
		return err
	}
	mw := authz.New(
		authz.WithTracerProvider(downstreamAggr.TracerProvider),
		authz.WithTokenExtractor(authz.ExtractFromAuthorizationHeader()),
		authz.WithVerifyOptions(jws.WithKeyProvider(kp)),
		authz.WithValidateOptions(jwt.WithAudience(os.Getenv("AUTH0_AUDIENCE"))),
	)
	loaderAggregate, err := loaders.NewAggregate(liverGroupRepository, loaders.WithTracerProvider(downstreamAggr.TracerProvider))
	if err != nil {
		return err
	}
	downstreamApp, err := downstream.New(downstreamAggr.TracerProvider, downstreamAggr.MetricProvider, rootResolver, mw, loaderAggregate)
	if err != nil {
		return fmt.Errorf("downstream.New: %w", err)
	}
	upstreamApp, err := upstream.New(upstreamAggr.TracerProvider, upstreamAggr.MetricProvider, upstreamHTTPClient, fmt.Sprintf("http://localhost:%d", downstreamPort))
	if err != nil {
		return fmt.Errorf("upstream.New: %w", err)
	}
	servers := []*server{
		{
			label: "upstream",
			srv: &http.Server{
				Addr:    fmt.Sprintf(":%d", upstreamPort),
				Handler: upstreamApp.Handler(),
			},
		},
		{
			label: "downstream",
			srv: &http.Server{
				Addr:    fmt.Sprintf(":%d", downstreamPort),
				Handler: downstreamApp.Handler(),
				BaseContext: func(_ net.Listener) context.Context {
					return context.WithValue(context.Background(), oauth2.HTTPClient, downstreamHTTPClient)
				},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	ctx, logger := log.FromContext(ctx)
	defer logger.Sync()
	eg, ctx := errgroup.WithContext(ctx)
	go graceful(ctx, servers...)
	for _, srv := range servers {
		l := logger.With(zap.String("server", srv.label))
		srv := srv
		eg.Go(func() error {
			l.Info("server started", zap.String("addr", srv.srv.Addr))
			if err := srv.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("%s: %w", srv.label, err)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("! %+v", err)
		exitCode := 1
		if err, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = err.ExitCode()
		}
		os.Exit(exitCode)
	}
}

type server struct {
	label string
	srv   *http.Server
}

func graceful(ctx context.Context, servers ...*server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	ctx, logger := log.FromContext(ctx)
	logger.Info("received signal", zap.Stringer("signal", sig))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for _, srv := range servers {
		srv := srv
		l := logger.With(zap.String("server", srv.label))
		wg.Add(1)
		go func(srv *server) {
			if err := srv.srv.Shutdown(ctx); err != nil {
				l.Warn("failed to gracefully shutdown server", zap.Error(err))
			}
			defer wg.Done()
		}(srv)
	}
	logger.Info("shutdown server")
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
			logger.Info("failed to cleanup otel trace provider", zap.String("server", component), zap.Error(err))
		}
		if err := aggr.MetricProvider.Shutdown(ctx); err != nil {
			logger.Info("failed to cleanup otel metric provider", zap.String("server", component), zap.Error(err))
		}
	}
	return aggr, cleanup, nil
}
