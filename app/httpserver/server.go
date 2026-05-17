package httpserver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

// ErrServerClosed is returned by ListenAndServe after a clean Shutdown().
var ErrServerClosed = errors.New("http: server closed")

// Server is a minimal HTTP/1.1 server built on raw TCP. It accepts connections,
// parses HTTP requests by hand, and dispatches them to an http.Handler — the
// same interface used by net/http. This means handlers, middleware, and routers
// (including http.ServeMux) work identically with this server.
//
// Unlike net/http's Server, this implementation does NOT:
//   - Auto-inject Date, Server, Content-Type, or Content-Length headers
//   - Support TLS, HTTP/2, or chunked transfer encoding
//   - Buffer responses (writes go directly to the connection)
//
// It DOES:
//   - Accept concurrent connections (goroutine per connection)
//   - Support HTTP/1.1 keep-alive (multiple requests per connection)
//   - Propagate a base context to all request handlers
//   - Drain unread request bodies between keep-alive requests
//   - Support graceful shutdown via Shutdown(ctx)
type Server struct {
	Addr    string
	Handler http.Handler

	// BaseContext optionally provides the base context for all requests on
	// all connections. If nil, context.Background() is used. Cancelling this
	// context signals all in-flight handlers via r.Context().Done().
	BaseContext func(net.Listener) context.Context

	listener   net.Listener
	mu         sync.Mutex
	activeConn sync.WaitGroup
	inShutdown atomic.Bool
}

// ListenAndServe binds to the configured address and starts accepting
// connections. It blocks until the server is shut down or an unrecoverable
// error occurs. Returns ErrServerClosed after a clean Shutdown().
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	return s.serve(ln)
}

// serve runs the accept loop, spawning a goroutine per connection.
func (s *Server) serve(ln net.Listener) error {
	var baseCtx context.Context
	if s.BaseContext != nil {
		baseCtx = s.BaseContext(ln)
	} else {
		baseCtx = context.Background()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.inShutdown.Load() {
				return ErrServerClosed
			}
			return fmt.Errorf("accept: %w", err)
		}

		s.activeConn.Add(1)
		go s.handleConn(baseCtx, conn)
	}
}

// handleConn runs the HTTP/1.1 keep-alive loop for a single connection.
// It reads requests sequentially (HTTP/1.1 requires ordered responses on a
// single connection), dispatches each to the handler, then drains any unread
// body bytes before reading the next request.
func (s *Server) handleConn(baseCtx context.Context, conn net.Conn) {
	defer s.activeConn.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		req, err := ParseRequest(reader)
		if err != nil {
			return
		}

		req = req.WithContext(baseCtx)

		w := newResponseWriter(conn)
		s.Handler.ServeHTTP(w, req)

		if req.Body != nil {
			io.Copy(io.Discard, req.Body)
			req.Body.Close()
		}

		if req.Header.Get("Connection") == "close" {
			return
		}
	}
}

// Shutdown gracefully stops the server. It marks the server as shutting down,
// closes the listener (stopping new connections), then waits for all in-flight
// requests to complete. If the context expires before all connections drain,
// it returns the context error.
func (s *Server) Shutdown(ctx context.Context) error {
	s.inShutdown.Store(true)

	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		ln.Close()
	}

	done := make(chan struct{})
	go func() {
		s.activeConn.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
