package graceful

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type Server interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func StartServer(ctx context.Context, srv Server) error {
	go graceful(ctx, srv)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func graceful(ctx context.Context, srv Server) {
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
