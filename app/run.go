package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Per48edjes/CodeCrafters-Go-HTTP-Server/app/httpserver"
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
	if len(config.SupportedEncodings) == 0 {
		config.SupportedEncodings = []string{"gzip"}
	}

	srv := NewServer(config)

	server := &httpserver.Server{
		Addr:           net.JoinHostPort("0.0.0.0", "4221"),
		Handler:        srv,
		RequestTimeout: config.RequestTimeout,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		fmt.Fprintf(stdout, "Server listening on %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != httpserver.ErrServerClosed {
			fmt.Fprintf(stdout, "Server error: %s\n", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()

		fmt.Fprintf(stdout, "Shutting down server...\n")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(stdout, "Server forced to shutdown: %s\n", err)
		} else {
			fmt.Fprintf(stdout, "Server exited cleanly\n")
		}
	}()

	wg.Wait()
	return nil
}
