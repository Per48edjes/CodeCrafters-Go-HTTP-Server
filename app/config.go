package main

import (
	"flag"
	"fmt"
)

// parseConfig processes command-line arguments into a Config struct.
// Using flag.NewFlagSet (instead of the global flag.CommandLine) keeps the
// global state clean and makes it possible to call parseConfig from tests
// with different argument slices.
func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("http-server", flag.ContinueOnError)

	directory := fs.String("directory", "", "directory to serve files from")

	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	return Config{
		DataDirectory: *directory,
	}, nil
}
