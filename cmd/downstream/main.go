package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aereal/enjoy-opentelemetry/downstream"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	flag.StringVar(&downstreamPort, "downstream-port", "8081", "downstream server port")
	flag.StringVar(&deploymentEnv, "env", os.Getenv("APP_ENV"), "deployment environment")
	flag.StringVar(&serviceName, "service", os.Getenv("APP_SERVICE_NAME"), "service name")
	flag.BoolVar(&debug, "debug", envDebug != "", "debug mode")
}

func run() error {
	flag.Parse()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, xray.Propagator{}))
	setupCtx := context.Background()
	downstreamTracerProvider, cleanupDownstream, err := setupTracerProvider(setupCtx, "downstream")
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		defer cleanupDownstream(ctx)
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
	log.Printf("port=%s env=%s service=%s debug=%v", downstreamPort, deploymentEnv, serviceName, debug)
	downstreamApp, err := downstream.New(downstreamTracerProvider, stsClient)
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
	log.Printf("listening on %s", downstreamSrv.Addr)
	if err := downstreamSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
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

func graceful(ctx context.Context, srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	log.Printf("received signal: %q", sig)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("failed to gracefully shutdown server: %s", err)
	}
	log.Print("shutdown server")
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
			log.Printf("%s: failed to cleanup otel trace provider: %s", component, err)
		}
	}
	return tp, cleanup, nil
}
