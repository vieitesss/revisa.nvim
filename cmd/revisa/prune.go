package main

import (
	"flag"
	"fmt"
	"io"
)

func prune(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("prune", flag.ContinueOnError)
	fl.SetOutput(stderr)
	branch := fl.String("branch", "",
		"only delete resolved comments on this branch (default: all)")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"prune: unexpected argument %q", positional[0]))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.Git {
		if err := gitOnlyFlags(fl, "branch"); err != nil {
			return err
		}
	}
	pruned, err := p.Prune(*branch)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "pruned %d\n", pruned)
	return nil
}
