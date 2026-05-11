package main

import (
	"context"
	"fmt"
	"os"
)

// main is intentionally trivial. All real logic lives in run(), which takes
// OS-level dependencies as arguments and returns an error. This makes the
// entire program testable: test code can call run() directly with controlled
// arguments, stdout, and context — no need to spawn a subprocess.
func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
