package httpserver

import (
	"fmt"
	"net"
	"net/http"
)

// responseWriter implements http.ResponseWriter over a raw net.Conn.
//
// Unlike net/http's internal response writer, this does NOT:
//   - Auto-inject headers (Date, Server, Content-Length, Content-Type)
//   - Buffer writes (each Write call goes directly to the TCP connection)
//   - Sniff content type from the response body
//
// Handlers are responsible for setting all required headers explicitly,
// including Content-Length for keep-alive connections (without it, the client
// cannot determine where the response body ends).
//
// This type is not safe for concurrent use — it is owned by a single goroutine
// for the lifetime of one request (HTTP/1.1 is serial per connection).
type responseWriter struct {
	conn        net.Conn
	headers     http.Header
	wroteHeader bool
}

func newResponseWriter(conn net.Conn) *responseWriter {
	return &responseWriter{
		conn:    conn,
		headers: make(http.Header),
	}
}

// Header returns the response header map. Handlers call w.Header().Set(...)
// to add headers before WriteHeader or Write is called. Headers set after
// WriteHeader are ignored (they've already been flushed to the wire).
func (rw *responseWriter) Header() http.Header {
	return rw.headers
}

// WriteHeader sends the HTTP/1.1 status line and all accumulated headers to
// the connection. Subsequent calls are no-ops — the status and headers can
// only be sent once per response.
func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true

	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))
	rw.conn.Write([]byte(statusLine))

	for key, values := range rw.headers {
		for _, v := range values {
			rw.conn.Write([]byte(fmt.Sprintf("%s: %s\r\n", key, v)))
		}
	}

	rw.conn.Write([]byte("\r\n"))
}

// Write sends body bytes to the connection. If WriteHeader has not been called,
// it implicitly sends a 200 OK status first (matching net/http's behavior).
func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.conn.Write(data)
}
