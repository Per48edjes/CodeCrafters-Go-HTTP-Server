package httpserver

import (
	"fmt"
	"net"
	"net/http"
)

// responseWriter implements http.ResponseWriter over a raw net.Conn.
//
// Unlike net/http's internal response writer, this does NOT:
//   - Auto-inject Date, Server, or Content-Type headers
//   - Sniff content type from the response body
//
// It DOES:
//   - Auto-inject Content-Length: 0 when the handler writes no body
//   - Defer flushing headers until the body is written (or the handler returns)
//
// This allows the server to add Content-Length: 0 for bodiless responses,
// which is required for keep-alive connections — without it, the client cannot
// determine that the response has no body.
//
// This type is not safe for concurrent use — it is owned by a single goroutine
// for the lifetime of one request (HTTP/1.1 is serial per connection).
type responseWriter struct {
	conn        net.Conn
	headers     http.Header
	statusCode  int
	wroteHeader bool
	flushed     bool
}

func newResponseWriter(conn net.Conn) *responseWriter {
	return &responseWriter{
		conn:    conn,
		headers: make(http.Header),
	}
}

// Header returns the response header map. Handlers call w.Header().Set(...)
// to add headers before WriteHeader or Write is called. Headers set after
// flush are ignored (they've already been sent to the wire).
func (rw *responseWriter) Header() http.Header {
	return rw.headers
}

// WriteHeader records the status code for the response. The actual bytes are
// not sent until Write is called or finish() is called by the server after
// the handler returns. This deferral allows the server to inject
// Content-Length: 0 for bodiless responses.
func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.statusCode = statusCode
}

// Write sends body bytes to the connection. On the first call, it flushes
// the status line and headers. If WriteHeader has not been called, it
// implicitly sets status 200 (matching net/http's behavior).
func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	if !rw.flushed {
		rw.flush()
	}
	return rw.conn.Write(data)
}

// finish is called by the server after the handler returns. It ensures the
// response is fully sent — if the handler never called Write, it flushes the
// status line and headers with Content-Length: 0 so the client knows the
// response is complete.
func (rw *responseWriter) finish() {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	if !rw.flushed {
		if rw.headers.Get("Content-Length") == "" {
			rw.headers.Set("Content-Length", "0")
		}
		rw.flush()
	}
}

func (rw *responseWriter) flush() {
	rw.flushed = true

	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", rw.statusCode, http.StatusText(rw.statusCode))
	rw.conn.Write([]byte(statusLine))

	for key, values := range rw.headers {
		for _, v := range values {
			rw.conn.Write([]byte(fmt.Sprintf("%s: %s\r\n", key, v)))
		}
	}

	rw.conn.Write([]byte("\r\n"))
}
