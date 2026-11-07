package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/vieitesss/revisa.nvim/internal/project"
)

var errMigrationPartial = errors.New("migration left comments behind")

func migrate(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fl.SetOutput(stderr)
	branch := fl.String("branch", "",
		"branch for moved comments (default: the current branch)")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"migrate: unexpected argument %q", positional[0]))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.Git {
		return fmt.Errorf("migrate: %w", project.ErrNeedsGit)
	}
	if p.Root == "" {
		return errors.New("migrate: no working tree to walk")
	}
	*branch, err = commentBranch(p, "migrate", *branch)
	if err != nil {
		return err
	}
	dirs, err := p.Migrate(*branch)
	moved, partial := 0, false
	for _, dir := range dirs {
		for _, file := range dir.Clashes {
			fmt.Fprintf(stderr, "migrate: %s: comment id already "+
				"exists in the git store\n", file)
			partial = true
		}
		if dir.Err != nil {
			continue
		}
		if dir.NonComments {
			fmt.Fprintf(stderr,
				"migrate: non-comment files found in %s/\n", dir.Dir)
			partial = true
		}
		moved += dir.Moved
		if dir.Moved > 0 {
			word := "comments"
			if dir.Moved == 1 {
				word = "comment"
			}
			fmt.Fprintf(stdout, "moved %d %s from %s/\n",
				dir.Moved, word, dir.Dir)
		}
	}
	if err != nil {
		return err
	}
	if moved == 0 {
		fmt.Fprintln(stdout, "no .revisa/ comments to move")
	}
	if partial {
		return errMigrationPartial
	}
	return nil
}
