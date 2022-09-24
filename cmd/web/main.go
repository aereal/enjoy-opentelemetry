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
	"sync"
	"syscall"
	"time"

	"github.com/aereal/enjoy-opentelemetry/downstream"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aereal/enjoy-opentelemetry/upstream"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	downstreamApp, err := downstream.New(downstreamTracerProvider, stsClient)
	if err != nil {
		return fmt.Errorf("downstream.New: %w", err)
	}
	upstreamApp, err := upstream.New(upstreamTracerProvider)
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
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)
	go graceful(ctx, servers...)
	for _, srv := range servers {
		srv := srv
		eg.Go(func() error {
			log.Printf("%s: listening on %s", srv.label, srv.srv.Addr)
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
		log.Printf("! %+v", err)
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
	log.Printf("received signal: %q", sig)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for _, srv := range servers {
		srv := srv
		wg.Add(1)
		go func(srv *server) {
			if err := srv.srv.Shutdown(ctx); err != nil {
				log.Printf("%s: failed to gracefully shutdown server: %s", srv.label, err)
			}
			defer wg.Done()
		}(srv)
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
