package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/aereal/enjoy-opentelemetry/tracing"
	"github.com/dimfeld/httptreemux/v5"
	"go.opentelemetry.io/otel/trace"
)

func New(tp trace.TracerProvider, client *http.Client, downstreamOrigin string) (*App, error) {
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	if downstreamOrigin == "" {
		return nil, fmt.Errorf("downstreamOrigin is empty")
	}
	parsed, err := url.Parse(downstreamOrigin)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %w", err)
	}
	return &App{tp: tp, client: client, downstreamOrigin: parsed}, nil
}

type App struct {
	tp               trace.TracerProvider
	client           *http.Client
	downstreamOrigin *url.URL
}

func (*App) handleHealthCheck() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"upstream","ok":true}`)
	})
}

func (*App) handleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprintln(w, `{"name":"upstream","ok":true}`)
	})
}

type errorPayload struct {
	Error string `json:"error"`
}

type httpHeaders http.Header

var _ interface {
	json.Marshaler
} = (httpHeaders)(nil)

func (h httpHeaders) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	for name := range h {
		m[textproto.CanonicalMIMEHeaderKey(name)] = nil
	}
	for canonicalName := range m {
		vs := http.Header(h).Values(canonicalName)
		if len(vs) > 1 {
			m[canonicalName] = vs
		} else {
			m[canonicalName] = vs[0]
		}
	}
	return json.Marshal(m)
}

type proxyResponsePayload struct {
	Status  int         `json:"status"`
	Headers httpHeaders `json:"headers"`
	Body    string      `json:"body"`
}

func (app *App) handleProxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		proxyPath := r.URL.Query().Get("path")
		if proxyPath == "" {
			respondError(w, http.StatusBadRequest, "path query parameter is required")
			return
		}
		proxyURL, err := app.downstreamOrigin.Parse(proxyPath)
		if err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("malformed URL: %+v", err))
			return
		}
		egressReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL.String(), nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("cannot build egress request: %+v", err))
			return
		}
		resp, err := app.client.Do(egressReq)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send request: %+v", err))
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("cannot read response body: %+v", err))
			return
		}
		p := proxyResponsePayload{Status: resp.StatusCode, Body: string(body), Headers: httpHeaders(resp.Header)}
		_ = json.NewEncoder(w).Encode(p)
	})
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorPayload{Error: msg})
}

func (app *App) Handler() http.Handler {
	mux := httptreemux.NewContextMux()
	mux.UseHandler(tracing.Middleware(app.tp))
	mux.Handler(http.MethodGet, "/", app.handleRoot())
	mux.Handler(http.MethodGet, "/proxy", app.handleProxy())
	mux.Handler(http.MethodGet, "/-/health", app.handleHealthCheck())
	return mux
}
