package main

import (
	"context"
	"encoding/json"
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
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
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
	downstreamTracerProvider, cleanupDownstream, err := setupTracerProvider(setupCtx, "downstream")
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
	dbx, err := db.New(os.Getenv("DSN"), db.WithTracerProvider(downstreamTracerProvider))
	if err != nil {
		return fmt.Errorf("db.New: %w", err)
	}
	liverGroupRepository, err := domain.NewLiverGroupRepository(domain.WithDB(dbx), domain.WithTracerProvider(downstreamTracerProvider))
	if err != nil {
		return err
	}
	rootResolver, err := resolvers.New(liverGroupRepository, dbx)
	if err != nil {
		return fmt.Errorf("resolvers.New: %w", err)
	}
	rt := otelhttp.NewTransport(
		&tracing.ResourceOverriderRoundTripper{Base: http.DefaultTransport},
		otelhttp.WithTracerProvider(downstreamTracerProvider),
	)
	httpClient := &http.Client{Transport: rt}
	kp, err := oidcconfig.NewKeyProvider(
		oidcconfig.WithHTTPClient(httpClient),
		oidcconfig.WithIssuer(os.Getenv("AUTH0_ISSUER")),
		oidcconfig.WithTracerProvider(downstreamTracerProvider),
	)
	if err != nil {
		return err
	}
	mw := authz.New(
		authz.WithTracerProvider(downstreamTracerProvider),
		authz.WithTokenExtractor(authz.ExtractFromAuthorizationHeader()),
		authz.WithVerifyOptions(jws.WithKeyProvider(kp)),
		authz.WithValidateOptions(jwt.WithAudience(os.Getenv("AUTH0_AUDIENCE"))),
	)
	f, err := os.Open("./oauth2-client-config.json")
	if err != nil {
		return err
	}
	defer f.Close()
	var authConfig oauth2.Config
	if err := json.NewDecoder(f).Decode(&authConfig); err != nil {
		return err
	}
	downstreamApp, err := downstream.New(downstreamTracerProvider, rootResolver, "https://aereal.org/#enjoy-opentelemetry-graphql", mw, &authConfig)
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

func setupTracerProvider(ctx context.Context, component string) (*sdktrace.TracerProvider, func(context.Context), error) {
	opts := []tracing.Option{
		tracing.WithHTTPExporter(),
		tracing.WithDeploymentEnvironment(deploymentEnv),
		tracing.WithResourceName(fmt.Sprintf("%s-%s", serviceName, component)),
	}
	if debug {
		opts = append(opts, tracing.WithDebugExporter(os.Stderr))
	}
	tp, err := tracing.Setup(ctx, opts...)
	if err != nil {
		return nil, noop, fmt.Errorf("%s: tracing.Setup: %w", component, err)
	}
	cleanup := func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			_, logger := log.FromContext(ctx)
			logger.Error("failed to cleanup otel trace provider", zap.String("component", component), zap.Error(err))
		}
	}
	return tp, cleanup, nil
}
