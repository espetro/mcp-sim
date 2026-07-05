package http

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// Server wraps an HTTP server with graceful shutdown.
type Server struct {
	Addr            string
	Handler         http.Handler
	ShutdownTimeout time.Duration
}

// New creates a new HTTP server.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		Addr:            addr,
		Handler:         handler,
		ShutdownTimeout: 10 * time.Second,
	}
}

// ListenAndServe starts the server and blocks until the context is cancelled.
// It handles SIGINT and SIGTERM for graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           s.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Wire healthz endpoint.
	mux, ok := s.Handler.(*http.ServeMux)
	if !ok {
		mux = http.NewServeMux()
		mux.Handle("/", s.Handler)
	}
	mux.HandleFunc("/healthz", healthz)

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	// Graceful shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// StartWithSignals starts the server and reacts to SIGINT/SIGTERM.
func (s *Server) StartWithSignals() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return s.ListenAndServe(ctx)
}
