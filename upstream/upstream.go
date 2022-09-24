package upstream

import (
	"fmt"
	"net/http"

	"github.com/dimfeld/httptreemux/v5"
)

func New() (*App, error) {
	return &App{}, nil
}

type App struct{}

func (*App) handleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"upstream","ok":true}`)
	})
}

func (app *App) Handler() http.Handler {
	mux := httptreemux.NewContextMux()
	mux.Handler(http.MethodGet, "/", app.handleRoot())
	return mux
}
