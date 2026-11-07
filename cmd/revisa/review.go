package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	command := "+Revisa review"
	if len(positional) == 1 {
		command += " " + exEscape(positional[0])
	}
	cmd := exec.Command(reviewEditor(), command)
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

var exEscaper = strings.NewReplacer(
	`\`, `\\`,
	" ", `\ `,
	"|", `\|`,
	`"`, `\"`,
)

// exEscape quotes one argument of an Ex command line.
func exEscape(argument string) string {
	return exEscaper.Replace(argument)
}
