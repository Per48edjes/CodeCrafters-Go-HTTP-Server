package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /echo/{str}", func(w http.ResponseWriter, r *http.Request) {
		str := r.PathValue("str")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(str)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(str))
	})

	mux.HandleFunc("GET /user-agent", func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(ua)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(ua))
	})

	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("finished"))
		case <-r.Context().Done():
			return
		}
	})

	server := &http.Server{
		Addr:    "0.0.0.0:4221",
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return baseCtx
		},
	}

	go func() {
		fmt.Println("Server listening on 0.0.0.0:4221")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Server error:", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	// Notify all in-flight handlers to stop
	baseCancel()

	// Wait up to 5 seconds for handlers to observe cancellation and exit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Println("Server forced to shutdown:", err)
		os.Exit(1)
	}
	fmt.Println("Server exited cleanly")
}
