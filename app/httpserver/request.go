package httpserver

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ParseRequest reads raw HTTP/1.1 bytes from the reader and constructs a
// standard *http.Request that can be dispatched to any http.Handler or
// http.ServeMux.
//
// It parses the three components of an HTTP message (RFC 9112):
//
//  1. Request line:  METHOD /path HTTP/1.1\r\n
//  2. Headers:       Key: Value\r\n (repeated, terminated by blank line)
//  3. Body:          Bounded by Content-Length header (chunked not supported)
//
// The returned request's Body is an io.LimitReader bounded to Content-Length
// bytes. If no Content-Length is present, Body is http.NoBody. On a keep-alive
// connection, the caller must drain any unread body bytes before reading the
// next request to keep the stream aligned.
//
// Returns an error if the connection is closed (io.EOF), the request line is
// malformed, or headers cannot be parsed.
func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read request line: %w", err)
	}
	requestLine = strings.TrimRight(requestLine, "\r\n")

	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", requestLine)
	}

	method := parts[0]
	rawURL := parts[1]
	proto := parts[2]

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", rawURL, err)
	}

	headers := make(http.Header)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			break
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("malformed header: %q", line)
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		headers.Add(key, value)
	}

	var body io.ReadCloser = http.NoBody
	if cl := headers.Get("Content-Length"); cl != "" {
		length, err := strconv.Atoi(cl)
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length %q: %w", cl, err)
		}
		if length > 0 {
			body = io.NopCloser(io.LimitReader(reader, int64(length)))
		}
	}

	return &http.Request{
		Method:     method,
		URL:        parsedURL,
		Proto:      proto,
		Header:     headers,
		Body:       body,
		Host:       headers.Get("Host"),
		RequestURI: rawURL,
	}, nil
}
