package main

import (
	"context"
	"net/http"
	"strings"
)

// Middleware is the standard shape for all middleware constructors in this
// server: a function that wraps an http.Handler with additional behavior.
// Every with* function returns this type, ensuring uniform wiring in NewServer:
//
//	handler = withContentEncoding(config.SupportedEncodings)(handler)
//	handler = withContextGuard()(handler)
type Middleware func(http.Handler) http.Handler

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

// withContextGuard returns middleware that short-circuits request handling if
// the server's base context has already been cancelled. This prevents handlers
// from starting work during the shutdown window — any request that arrives after
// the shutdown signal will get an empty response rather than beginning work it
// can't finish.
//
// For handlers that do multi-step work (disk I/O, network calls), the handler
// itself should additionally check r.Context().Err() between steps. This
// middleware handles the coarse case; fine-grained cancellation is the handler's
// responsibility.
func withContextGuard() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Context().Err() != nil {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// withContentEncoding returns middleware that negotiates content encoding for
// each request. The supported parameter is the server's canonical set of
// encodings it can produce (e.g., []string{"gzip"}). The middleware:
//  1. Computes the intersection of client-accepted and server-supported encodings
//  2. Stashes that intersection in the request context (accessible via SupportedEncodings)
//  3. Sets the Content-Encoding response header to the first negotiated encoding
//
// Handlers inherit the Content-Encoding default but have final say — they can
// override or delete the header before writing the response body, using
// SupportedEncodings(r.Context()) to see what's safe to pick from.
func withContentEncoding(supported []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			negotiated := negotiateEncodings(r, supported)

			ctx := context.WithValue(r.Context(), encodingsKey, negotiated)
			r = r.WithContext(ctx)

			if len(negotiated) > 0 {
				w.Header().Set("Content-Encoding", negotiated[0])
			}

			next.ServeHTTP(w, r)
		})
	}
}

// negotiateEncodings computes the intersection of the client's Accept-Encoding
// tokens and the server's supported set. Only encodings that appear in both are
// returned — this guarantees the client can decode whatever we send. The order
// follows the client's Accept-Encoding list, filtered by server support.
func negotiateEncodings(r *http.Request, supported []string) []string {
	accepted := r.Header.Get("Accept-Encoding")
	if accepted == "" {
		return nil
	}

	var negotiated []string
	for _, token := range strings.Split(accepted, ",") {
		token = strings.TrimSpace(token)
		for _, s := range supported {
			if token == s {
				negotiated = append(negotiated, token)
				break
			}
		}
	}

	return negotiated
}
