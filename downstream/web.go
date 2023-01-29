package downstream

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/aereal/enjoy-opentelemetry/authz"
	"github.com/aereal/enjoy-opentelemetry/graph"
	"github.com/aereal/enjoy-opentelemetry/graph/cache"
	"github.com/aereal/enjoy-opentelemetry/graph/directives"
	"github.com/aereal/enjoy-opentelemetry/graph/loaders"
	"github.com/aereal/enjoy-opentelemetry/graph/resolvers"
	"github.com/aereal/enjoy-opentelemetry/tracing"
	otelgqlgen "github.com/aereal/otelgqlgen"
	"github.com/dimfeld/httptreemux/v5"
	"github.com/rs/cors"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"
)

func New(tp trace.TracerProvider, rootResolver *resolvers.Resolver, audience string, authenticator *authz.Middleware, authConfig *oauth2.Config, loaderAggregate *loaders.Aggregate) (*App, error) {
	if rootResolver == nil {
		return nil, errors.New("rootResolver is nil")
	}
	tracer := tp.Tracer("downstream")
	return &App{
		tp:              tp,
		tracer:          tracer,
		resolver:        rootResolver,
		audience:        audience,
		authenticator:   authenticator,
		authConfig:      authConfig,
		loaderAggregate: loaderAggregate,
	}, nil
}

type App struct {
	tp              trace.TracerProvider
	tracer          trace.Tracer
	resolver        *resolvers.Resolver
	audience        string
	authenticator   *authz.Middleware
	authConfig      *oauth2.Config
	loaderAggregate *loaders.Aggregate
}

func (*App) handleHealthCheck() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"downstream","ok":true}`)
	})
}

func (a *App) handleGraphql() http.Handler {
	cfg := graph.Config{
		Resolvers:  a.resolver,
		Directives: directives.New(directives.WithTracerProvider(a.tp)),
	}
	srv := handler.New(graph.NewExecutableSchema(cfg))
	srv.SetQueryCache(cache.NewTracedCache(lru.New(100), cache.WithTracerProvider(a.tp)))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(otelgqlgen.New(otelgqlgen.WithTracerProvider(a.tp)))
	srv.Use(a.loaderAggregate)
	return srv
}

func (*App) handleRoot() http.Handler {
	return playground.Handler("GraphQL playground", "/graphql")
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
	opts := cors.Options{}
	opts.AllowCredentials = true
	opts.AllowedHeaders = append(opts.AllowedHeaders, "authorization", "content-type")
	opts.AllowOriginFunc = func(origin string) bool {
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return parsed.Hostname() == "localhost"
	}
	corsMW := cors.New(opts)
	router := httptreemux.NewContextMux()
	router.OptionsHandler = func(w http.ResponseWriter, r *http.Request, m map[string]string) {
		corsMW.ServeHTTP(w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	}
	router.UseHandler(tracing.Middleware(app.tp))
	router.Handler(http.MethodGet, "/", app.handleRoot())
	router.Handler(http.MethodGet, "/-/health", app.handleHealthCheck())
	router.Handler(http.MethodGet, "/signin", app.handleSignIn())
	router.Handler(http.MethodGet, "/auth/callback", app.handleCallback())
	router.Handler(http.MethodPost, "/graphql", corsMW.Handler(app.authenticator.Authenticate(app.handleGraphql())))
	return router
}
