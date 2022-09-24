package upstream

import (
	"fmt"
	"net/http"

	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/otel/trace"
)

func New(tp trace.TracerProvider) (*App, error) {
	return &App{tp: tp}, nil
}

type App struct {
	tp trace.TracerProvider
}

func (*App) handleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"upstream","ok":true}`)
	})
}

func (app *App) Handler() http.Handler {
	mux := httptreemux.NewContextMux()
	mux.UseHandler(tracing.Middleware(app.tp))
	mux.Handler(http.MethodGet, "/", app.handleRoot())
	return mux
}
