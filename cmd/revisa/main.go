package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errUsageReported) {
			fmt.Fprintln(os.Stderr, "revisa:", err)
		}
		os.Exit(1)
	}
}
