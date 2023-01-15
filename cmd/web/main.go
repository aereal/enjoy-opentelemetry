package main

import (
	"context"
	"encoding/json"
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
	"github.com/aereal/enjoy-opentelemetry/downstream"
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aereal/enjoy-opentelemetry/upstream"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	upstreamTracerProvider, cleanupUpstream, err := setupTracerProvider(setupCtx, "upstream")
	if err != nil {
		return err
	}
	downstreamTracerProvider, cleanupDownstream, err := setupTracerProvider(setupCtx, "downstream")
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		defer cleanupDownstream(ctx)
		defer cleanupUpstream(ctx)
	}()
	cfg, err := config.LoadDefaultConfig(
		setupCtx,
		config.WithEndpointDiscovery(aws.EndpointDiscoveryDisabled),
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled),
		config.WithEC2RoleCredentialOptions(nil),
	)
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	otelaws.AppendMiddlewares(&cfg.APIOptions, otelaws.WithTracerProvider(downstreamTracerProvider))
	stsClient := sts.NewFromConfig(cfg)
	dbx, err := db.New(downstreamTracerProvider, os.Getenv("DSN"))
	if err != nil {
		return fmt.Errorf("db.New: %w", err)
	}
	rootResolver, err := resolvers.New(dbx)
	if err != nil {
		return fmt.Errorf("resolvers.New: %w", err)
	}
	baseTransport := &tracing.ResourceOverriderRoundTripper{Base: http.DefaultTransport}
	upstreamHTTPClient := &http.Client{
		Transport: otelhttp.NewTransport(baseTransport, otelhttp.WithTracerProvider(upstreamTracerProvider)),
	}
	downstreamHTTPClient := &http.Client{
		Transport: otelhttp.NewTransport(baseTransport, otelhttp.WithTracerProvider(downstreamTracerProvider)),
	}
	kp, err := oidcconfig.NewKeyProvider(
		oidcconfig.WithHTTPClient(downstreamHTTPClient),
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
	downstreamApp, err := downstream.New(downstreamTracerProvider, stsClient, rootResolver, "https://aereal.org/#enjoy-opentelemetry-graphql", mw, &authConfig)
	if err != nil {
		return fmt.Errorf("downstream.New: %w", err)
	}
	upstreamApp, err := upstream.New(upstreamTracerProvider, upstreamHTTPClient, fmt.Sprintf("http://localhost:%d", downstreamPort))
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
			logger.Info("failed to cleanup otel trace provider", zap.String("server", component), zap.Error(err))
		}
	}
	return tp, cleanup, nil
}
