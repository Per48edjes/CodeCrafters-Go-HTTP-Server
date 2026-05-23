package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
)

// gzipBytes compresses data using gzip and returns the compressed bytes.
func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	return buf.Bytes(), nil
}
