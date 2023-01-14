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
	"github.com/aereal/enjoy-opentelemetry/authz"
	"github.com/aereal/enjoy-opentelemetry/graph"
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	otelgqlgen "github.com/aereal/otelgqlgen"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func New(tp trace.TracerProvider, stsClient *sts.Client, rootResolver *resolvers.Resolver, audience string, authenticator *authz.Middleware, authConfig *oauth2.Config) (*App, error) {
	if stsClient == nil {
		return nil, errors.New("stsClient is nil")
	}
	if rootResolver == nil {
		return nil, errors.New("rootResolver is nil")
	}
	tracer := tp.Tracer("downstream")
	return &App{
		stsClient:     stsClient,
		tp:            tp,
		tracer:        tracer,
		resolver:      rootResolver,
		audience:      audience,
		authenticator: authenticator,
		authConfig:    authConfig,
	}, nil
}

type App struct {
	stsClient     *sts.Client
	tp            trace.TracerProvider
	tracer        trace.Tracer
	resolver      *resolvers.Resolver
	audience      string
	authenticator *authz.Middleware
	authConfig    *oauth2.Config
}

func (*App) handleHealthCheck() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"downstream","ok":true}`)
	})
}

func (a *App) handleGraphql() http.Handler {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{
		Resolvers: a.resolver,
	}))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(otelgqlgen.New(otelgqlgen.WithTracerProvider(a.tp)))
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

func (app *App) handleSignIn() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := app.authConfig.AuthCodeURL("0xdeadbeaf", oauth2.SetAuthURLParam("audience", app.audience))
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})
}

func (app *App) handleCallback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := app.authConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/?token="+token.AccessToken, http.StatusTemporaryRedirect)
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
	router.Handler(http.MethodGet, "/signin", app.handleSignIn())
	router.Handler(http.MethodGet, "/auth/callback", app.handleCallback())
	router.Handler(http.MethodPost, "/graphql", app.authenticator.Authenticate(app.handleGraphql()))
	return router
}
