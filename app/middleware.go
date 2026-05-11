package main

import "net/http"

// withContextGuard is middleware that short-circuits request handling if the
// server's base context has already been cancelled. This prevents handlers from
// starting work during the shutdown window — any request that arrives after the
// shutdown signal will get an empty response rather than beginning work it can't finish.
//
// For handlers that do multi-step work (disk I/O, network calls), the handler
// itself should additionally check r.Context().Err() between steps. This
// middleware handles the coarse case; fine-grained cancellation is the handler's
// responsibility.
//
// In a production server, additional middleware would be layered here:
//   - panicRecovery: catch panics in handlers, log a stack trace, return 500
//   - requestID: generate or propagate a unique ID for distributed tracing
//   - logging: structured request/response logging (method, path, status, duration)
//   - cors: set Access-Control-* headers for browser clients
//   - rateLimit: protect the server from excessive traffic
func withContextGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != nil {
			return
		}
		next.ServeHTTP(w, r)
	})
}
