package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// handleIndex returns a handler for the root path. In a real service this
// might serve a health check page or redirect to API docs.
func handleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// handleEcho returns a handler that echoes back the path parameter as
// plain text. Demonstrates path parameter extraction via r.PathValue().
func handleEcho() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		str := r.PathValue("str")

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(str)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(str))
	})
}

// handleUserAgent returns a handler that reads the User-Agent request header
// and echoes it back as the response body. Demonstrates reading request headers.
func handleUserAgent() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(ua)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(ua))
	})
}

// handleGetFile returns a handler that serves files from the given directory.
// The directory is passed as an explicit dependency — the handler doesn't reach
// into global state or config to find it.
//
// In a production server you'd want to:
//   - Sanitize the filename to prevent path traversal (filepath.Join helps here)
//   - Set appropriate cache headers
//   - Support Range requests for large files
//   - Consider streaming via io.Copy instead of reading the full file into memory
func handleGetFile(directory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		path := filepath.Join(directory, filename)

		// Check context between resolving the path and performing disk I/O.
		// If the server is shutting down, don't bother reading the file.
		if r.Context().Err() != nil {
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
}

// handlePostFile returns a handler that writes the request body to a file in
// the given directory. The filename comes from the URL path parameter.
func handlePostFile(directory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		path := filepath.Join(directory, filename)

		if r.Context().Err() != nil {
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})
}

// handleSlow returns a handler that simulates a long-running operation.
// It demonstrates cooperative cancellation: the handler respects r.Context()
// and bails out cleanly when the server shuts down or the client disconnects,
// rather than blocking for the full duration.
func handleSlow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
}
