//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The client is Windows-only; this stub keeps `go build ./...` and `go vet ./...`
// working on macOS and Linux, where the shared packages are tested.
func main() {
	fmt.Fprintln(os.Stderr, "AI Passport Windows 7 edition: build with GOOS=windows")
	os.Exit(1)
}
