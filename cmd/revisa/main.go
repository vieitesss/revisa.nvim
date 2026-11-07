package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errMigrationPartial) {
			os.Exit(3)
		}
		if !errors.Is(err, errUsageReported) {
			fmt.Fprintln(os.Stderr, "revisa:", err)
		}
		if _, ok := errors.AsType[usageError](err); ok {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
