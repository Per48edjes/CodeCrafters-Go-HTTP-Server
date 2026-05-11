package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// run is the real entrypoint for the server. It takes OS-level dependencies as
// arguments so the program is fully testable without spinning up a separate process.
// If run returns nil, the program exited cleanly. If it returns an error, main
// should print it and exit with a non-zero code.
func run(ctx context.Context, args []string, stdout io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	config, err := parseConfig(args)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	srv := NewServer(config)

	httpServer := &http.Server{
		Addr:    net.JoinHostPort("0.0.0.0", "4221"),
		Handler: srv,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	// Start the server in a separate goroutine so we can listen for the
	// cancellation signal in the main goroutine.
	go func() {
		fmt.Fprintf(stdout, "Server listening on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stdout, "Server error: %s\n", err)
		}
	}()

	// Block until the context is cancelled (OS signal received).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()

		fmt.Fprintf(stdout, "Shutting down server...\n")

		// Give in-flight requests a deadline to complete. Handlers observing
		// r.Context().Done() will be notified because the base context is already
		// cancelled at this point (it's the same ctx we derived from signal.NotifyContext).
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(stdout, "Server forced to shutdown: %s\n", err)
		}
	}()

	wg.Wait()
	fmt.Fprintf(stdout, "Server exited cleanly\n")
	return nil
}
