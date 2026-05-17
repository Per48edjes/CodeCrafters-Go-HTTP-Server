package httpserver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ErrServerClosed is returned by ListenAndServe after a clean Shutdown().
var ErrServerClosed = errors.New("http: server closed")

// Server is a minimal HTTP/1.1 server built on raw TCP. It accepts connections,
// parses HTTP requests by hand, and dispatches them to an http.Handler — the
// same interface used by net/http. This means you can swap between this server
// and http.Server without changing any handler, middleware, or routing code.
//
// Unlike net/http's Server, this implementation does NOT:
//   - Auto-inject Date, Server, or Content-Length headers
//   - Support keep-alive (closes connection after each response)
//   - Support TLS, HTTP/2, or chunked transfer encoding
//
// It DOES:
//   - Accept concurrent connections (goroutine per connection)
//   - Propagate a base context to all request handlers
//   - Support graceful shutdown via Shutdown(ctx)
type Server struct {
	Addr    string
	Handler http.Handler

	// BaseContext optionally provides the base context for all requests.
	// If nil, context.Background() is used.
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

func (s *Server) handleConn(baseCtx context.Context, conn net.Conn) {
	defer s.activeConn.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		if baseCtx.Err() != nil {
			return
		}

		// Set a short read deadline so we don't block forever waiting for
		// the next request. If the deadline fires, we check whether shutdown
		// is in progress. If not, we extend the deadline and try again.
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		req, err := ParseRequest(reader)
		if err != nil {
			if baseCtx.Err() != nil {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		// Clear the deadline for handler execution — handlers may do
		// slow work and shouldn't be constrained by the keep-alive poll interval.
		conn.SetReadDeadline(time.Time{})

		req = req.WithContext(baseCtx)

		w := newResponseWriter(conn)
		s.Handler.ServeHTTP(w, req)

		if req.Header.Get("Connection") == "close" {
			return
		}
	}
}

// Shutdown gracefully stops the server. It closes the listener (no new
// connections), then waits for all in-flight requests to complete or for
// the context to expire — whichever comes first.
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
