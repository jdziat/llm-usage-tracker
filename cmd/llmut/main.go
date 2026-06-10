package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jdziat/llm-usage-tracker/internal/usage"
)

func main() {
	if err := usage.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		var exitErr usage.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
