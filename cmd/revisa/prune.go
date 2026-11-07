package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/vieitesss/revisa.nvim/internal/store"
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
	all, err := store.Load(p.Store)
	if err != nil {
		return err
	}
	pruned := 0
	for _, c := range all {
		if c.Status != "resolved" {
			continue
		}
		if *branch != "" && c.Branch != *branch {
			continue
		}
		if err := store.Delete(p.Store, c.ID); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
		pruned++
	}
	fmt.Fprintf(stdout, "pruned %d\n", pruned)
	return nil
}
