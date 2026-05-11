package main

import "net/http"

// addRoutes is the single source of truth for the API surface of this service.
// Every route is registered here so you can glance at one file and understand
// what endpoints exist, what methods they accept, and what middleware wraps them.
//
// When the service grows, keep this function flat and readable. Resist the
// temptation to scatter route registration across multiple files.
func addRoutes(mux *http.ServeMux, config Config) {
	mux.Handle("GET /{$}", handleIndex())
	mux.Handle("GET /echo/{str}", handleEcho())
	mux.Handle("GET /user-agent", handleUserAgent())
	mux.Handle("GET /files/{filename}", handleGetFile(config.DataDirectory))
	mux.Handle("GET /slow", handleSlow())
}
