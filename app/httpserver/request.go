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
// standard *http.Request. This is the hand-rolled equivalent of what net/http
// does internally when it reads from a connection.
//
// HTTP/1.1 request wire format (RFC 9112):
//
//	GET /echo/hello HTTP/1.1\r\n      ← request line
//	Host: localhost:4221\r\n           ← headers (one per line)
//	Content-Length: 5\r\n
//	\r\n                               ← blank line marks end of headers
//	hello                              ← body (optional, length from Content-Length)
func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	// --- Parse the request line ---
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

	// --- Parse headers ---
	headers := make(http.Header)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")

		// Empty line marks end of headers
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

	// --- Parse body ---
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

	req := &http.Request{
		Method:     method,
		URL:        parsedURL,
		Proto:      proto,
		Header:     headers,
		Body:       body,
		Host:       headers.Get("Host"),
		RequestURI: rawURL,
	}

	return req, nil
}
