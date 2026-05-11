package main

import "net/http"

// Config holds all configuration for the server, parsed from command-line
// flags or environment variables. Keeping it in a struct makes it easy to
// pass around and test with different values.
type Config struct {
	DataDirectory string
}

// NewServer is the top-level constructor for the HTTP service. It takes all
// dependencies as explicit arguments and returns an http.Handler ready to be
// mounted on an http.Server.
//
// This is the place to wire up cross-cutting middleware that applies to every
// request (logging, tracing, auth, CORS, panic recovery, etc.). Route-specific
// middleware belongs in routes.go alongside the handler registration.
func NewServer(config Config) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, config)

	// Apply global middleware. Order matters: the outermost wrapper runs first.
	// In a production server you'd typically layer these:
	//   handler = panicRecovery(handler)
	//   handler = requestID(handler)
	//   handler = logging(handler)
	//   handler = cors(handler)
	var handler http.Handler = mux
	handler = withContextGuard(handler)

	return handler
}
