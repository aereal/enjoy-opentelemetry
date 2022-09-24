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
	"golang.org/x/sync/errgroup"
)

var (
	shutdownTimeout  = time.Second * 5
	httpClient       = otelhttp.DefaultClient
	attrResourceName = attribute.Key("resource.name")

	upstreamPort   int
	downstreamPort int
	deploymentEnv  string
)

func init() {
	flag.IntVar(&upstreamPort, "upstream-port", 8080, "upstream server port")
	flag.IntVar(&downstreamPort, "downstream-port", 8081, "downstream server port")
	flag.StringVar(&deploymentEnv, "env", "local", "deployment environment")
}

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
	flag.Parse()
	setupCtx := context.Background()
	tp, err := tracing.Setup(
		setupCtx,
		tracing.WithDebugExporter(os.Stderr),
		tracing.WithHTTPExporter(),
		tracing.WithDeploymentEnvironment(deploymentEnv),
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
	servers := []*server{
		{
			label: "upstream",
			srv: &http.Server{
				Addr:    fmt.Sprintf(":%d", upstreamPort),
				Handler: buildUpstreamHandler(),
			},
		},
		{
			label: "downstream",
			srv: &http.Server{
				Addr:    fmt.Sprintf(":%d", downstreamPort),
				Handler: withTrace()(downstreamApp.Handler()),
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

func buildUpstreamHandler() http.Handler {
	mux := httptreemux.NewContextMux()
	mux.GET("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"upstream","ok":true}`)
	}))
	return mux
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
