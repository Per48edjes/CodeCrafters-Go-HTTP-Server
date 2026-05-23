package main

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a private struct type used as context.Value keys. Using a named
// struct (rather than a bare string) ensures key uniqueness via pointer identity:
// no other package can produce a value that == matches our package-level var,
// even if they use the same string internally. The name field is purely for
// debugging (e.g., fmt.Sprintf("%v", key)).
type contextKey struct{ name string }

// encodingsKey is the context lookup key for the negotiated encoding set.
// Middleware stores the intersection of client-accepted and server-supported
// encodings under this key; handlers retrieve it via SupportedEncodings().
var encodingsKey = &contextKey{"supportedEncodings"}

// serverSupportedEncodings is the canonical set of content encodings this server
// can produce. The content encoding middleware intersects this with the client's
// Accept-Encoding header to determine which encodings are safe to advertise.
// To add support for a new encoding (e.g., "br"), append it here and implement
// the corresponding compression in the response path.
var serverSupportedEncodings = []string{"gzip"}

// SupportedEncodings returns the set of content encodings negotiated for this
// request — i.e., encodings that both the client accepts AND the server supports.
// Handlers call this to see the "menu" of valid encodings they may choose from
// when overriding the middleware's default selection.
//
// Returns nil if the middleware has not run or no encodings were negotiated.
// The type assertion is necessary because context.Value returns an untyped
// interface{}; this accessor hides that machinery behind a typed API.
func SupportedEncodings(ctx context.Context) []string {
	if v, ok := ctx.Value(encodingsKey).([]string); ok {
		return v
	}
	return nil
}

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

// withContentEncoding negotiates content encoding for each request. It:
//  1. Computes the intersection of client-accepted and server-supported encodings
//  2. Stashes that intersection in the request context (accessible via SupportedEncodings)
//  3. Sets the Content-Encoding response header to the server's preferred encoding
//
// Handlers inherit the Content-Encoding default but have final say — they can
// override or delete the header before writing the response body, using
// SupportedEncodings(r.Context()) to see what's safe to pick from.
func withContentEncoding(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		negotiated := negotiateEncodings(r)

		ctx := context.WithValue(r.Context(), encodingsKey, negotiated)
		r = r.WithContext(ctx)

		if encoding := preferEncoding(negotiated); encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
		}

		next.ServeHTTP(w, r)
	})
}

// negotiateEncodings computes the intersection of the client's Accept-Encoding
// tokens and serverSupportedEncodings. Only encodings that appear in both sets
// are returned — this guarantees the client can decode whatever we send.
func negotiateEncodings(r *http.Request) []string {
	accepted := r.Header.Get("Accept-Encoding")
	if accepted == "" {
		return nil
	}

	var negotiated []string
	for _, token := range strings.Split(accepted, ",") {
		token = strings.TrimSpace(token)
		for _, supported := range serverSupportedEncodings {
			if token == supported {
				negotiated = append(negotiated, token)
				break
			}
		}
	}

	return negotiated
}

// preferEncoding applies the server's preference policy over the negotiated set.
// Currently: prefer gzip if available, otherwise no encoding (identity).
func preferEncoding(negotiated []string) string {
	for _, enc := range negotiated {
		if enc == "gzip" {
			return "gzip"
		}
	}
	return ""
}
