package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// review starts Neovim in the worktree picker, keeping the working
// directory. REVISA_NVIM chooses another editor command.
func review(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("review", flag.ContinueOnError)
	fl.SetOutput(stderr)
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 1 {
		return usageError(fmt.Sprintf(
			"review: unexpected argument %q", positional[1]))
	}
	worktree := ""
	if len(positional) == 1 {
		worktree = positional[0]
	}
	cmd := exec.Command(reviewEditor(), "+Revisa review")
	// The worktree travels in the environment, which carries it verbatim.
	// An Ex command line cannot: Neovim reads backslashes and bars in it
	// before the command runs, so a path with a space or a pipe would
	// reach the plugin mangled. Always set it, so a stale value in the
	// caller's environment cannot name another worktree.
	cmd.Env = append(os.Environ(), "REVISA_REVIEW_WORKTREE="+worktree)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("review: %w", err)
	}
	return nil
}

func reviewEditor() string {
	if name := os.Getenv("REVISA_NVIM"); name != "" {
		return name
	}
	return "nvim"
}
