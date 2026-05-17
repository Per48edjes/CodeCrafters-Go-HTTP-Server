package main

import (
	"flag"
	"fmt"
	"time"
)

// Config holds all configuration for the server, parsed from command-line
// flags or environment variables. Keeping it in a struct makes it easy to
// pass around and test with different values.
type Config struct {
	DataDirectory  string
	RequestTimeout time.Duration
}

// parseConfig processes command-line arguments into a Config struct.
// Using flag.NewFlagSet (instead of the global flag.CommandLine) keeps the
// global state clean and makes it possible to call parseConfig from tests
// with different argument slices.
func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("http-server", flag.ContinueOnError)

	directory := fs.String("directory", "", "directory to serve files from")
	requestTimeout := fs.Duration("request-timeout", 30*time.Second, "maximum duration for a single request")

	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	return Config{
		DataDirectory:  *directory,
		RequestTimeout: *requestTimeout,
	}, nil
}
