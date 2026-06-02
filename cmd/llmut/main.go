package main

import (
	"fmt"
	"os"

	"github.com/jdziat/llm-usage-tracker/internal/usage"
)

func main() {
	if err := usage.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
