package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"sync"
)

// responseWriter implements http.ResponseWriter over a raw net.Conn.
// Unlike net/http's internal implementation, this does NOT auto-inject
// Date, Server, Content-Length, or any other headers. Handlers get exactly
// the response they write — nothing more.
type responseWriter struct {
	conn          net.Conn
	headers       http.Header
	wroteHeader   bool
	statusCode    int
	headerWritten bool
	mu            sync.Mutex
}

func newResponseWriter(conn net.Conn) *responseWriter {
	return &responseWriter{
		conn:    conn,
		headers: make(http.Header),
	}
}

// Header returns the response header map. Handlers call this to set headers
// before WriteHeader or Write is called.
func (rw *responseWriter) Header() http.Header {
	return rw.headers
}

// WriteHeader sends the HTTP status line and headers to the connection.
// It can only be called once — subsequent calls are no-ops.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.statusCode = statusCode

	// Write status line: "HTTP/1.1 200 OK\r\n"
	statusText := http.StatusText(statusCode)
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText)
	rw.conn.Write([]byte(statusLine))

	// Write headers: "Key: Value\r\n" for each
	for key, values := range rw.headers {
		for _, v := range values {
			rw.conn.Write([]byte(fmt.Sprintf("%s: %s\r\n", key, v)))
		}
	}

	// End of headers
	rw.conn.Write([]byte("\r\n"))
}

// Write writes body bytes to the connection. If WriteHeader hasn't been
// called yet, it implicitly calls WriteHeader(200) first — matching the
// behavior of net/http's ResponseWriter.
func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.conn.Write(data)
}
