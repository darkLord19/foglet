package main

import (
	"fmt"
	"os"
)

func main() {
	_, _ = fmt.Fprintln(os.Stderr, unsupportedBuildMessage())
	_, _ = fmt.Fprintln(os.Stderr, "Use: wails dev -tags desktop (or go run -tags desktop ./cmd/fogapp)")
	os.Exit(1)
}

func unsupportedBuildMessage() string {
	return "fogapp desktop build requires Wails and build tag 'desktop'."
}
