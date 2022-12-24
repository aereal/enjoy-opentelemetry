package downstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/aereal/enjoy-opentelemetry/graph"
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dimfeld/httptreemux/v5"
	"github.com/ravilushqa/otelgqlgen"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func New(tp trace.TracerProvider, stsClient *sts.Client) (*App, error) {
	if stsClient == nil {
		return nil, errors.New("stsClient is nil")
	}
	tracer := tp.Tracer("downstream")
	return &App{stsClient: stsClient, tp: tp, tracer: tracer}, nil
}

type App struct {
	stsClient *sts.Client
	tp        trace.TracerProvider
	tracer    trace.Tracer
}

func (*App) handleHealthCheck() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"downstream","ok":true}`)
	})
}

func (a *App) handleGraphql() http.Handler {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{
		Resolvers: &resolvers.Resolver{},
	}))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(otelgqlgen.Middleware(otelgqlgen.WithTracerProvider(a.tp)))
	return srv
}

func (*App) handleRoot() http.Handler {
	return playground.Handler("GraphQL playground", "/graphql")
}

func (app *App) handleUser() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		data := struct{ ID string }{}
		ctx, logger := log.FromContext(r.Context())
		logger.Info("handle /users/:id", zap.String("xray_trace_id", r.Header.Get("X-Amzn-Trace-Id")))
		params := httptreemux.ContextParams(ctx)
		if id, ok := params["id"]; ok {
			data.ID = id
		}
		_, span := app.tracer.Start(ctx, "createResponse")
		defer span.End()
		_ = json.NewEncoder(w).Encode(data)
	})
}

func (app *App) handleMe() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		ctx, logger := log.FromContext(r.Context())
		logger.Info("handle /me", zap.String("xray_trace_id", r.Header.Get("X-Amzn-Trace-Id")))
		out, err := app.stsClient.GetCallerIdentity(ctx, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		body := struct {
			Account string `json:",omitempty"`
			Arn     string `json:",omitempty"`
			UserID  string `json:",omitempty"`
		}{}
		if out.Account != nil {
			body.Account = *out.Account
		}
		if out.Arn != nil {
			body.Arn = *out.Arn
		}
		if out.UserId != nil {
			body.UserID = *out.UserId
		}
		_ = json.NewEncoder(w).Encode(body)
	})
}

type Router interface {
	Handler(method, path string, handler http.Handler)
}

func (app *App) Handler() http.Handler {
	router := httptreemux.NewContextMux()
	router.UseHandler(tracing.Middleware(app.tp))
	router.Handler(http.MethodGet, "/", app.handleRoot())
	router.Handler(http.MethodGet, "/me", app.handleMe())
	router.Handler(http.MethodGet, "/users/:id", app.handleUser())
	router.Handler(http.MethodGet, "/-/health", app.handleHealthCheck())
	router.Handler(http.MethodPost, "/graphql", app.handleGraphql())
	return router
}
