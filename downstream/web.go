package downstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/otel/trace"
)

func New(tp trace.TracerProvider, stsClient *sts.Client) (*App, error) {
	if stsClient == nil {
		return nil, errors.New("stsClient is nil")
	}
	return &App{stsClient: stsClient, tp: tp}, nil
}

type App struct {
	stsClient *sts.Client
	tp        trace.TracerProvider
}

func (*App) handleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"ok":true}`)
	})
}

func (*App) handleUser() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		data := struct{ ID string }{}
		params := httptreemux.ContextParams(r.Context())
		if id, ok := params["id"]; ok {
			data.ID = id
		}
		_ = json.NewEncoder(w).Encode(data)
	})
}

func (app *App) handleMe() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		ctx := r.Context()
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
	return router
}
