// Package git runs the git commands revisa needs, reporting git's own
// message when one fails.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNotRepo reports discovery outside any git repository.
var ErrNotRepo = errors.New("not a git repository")

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", commandError(err)
	}
	return string(out), nil
}

func commandError(err error) error {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return errors.New(strings.TrimSpace(string(exit.Stderr)))
	}
	return fmt.Errorf("run git: %w", err)
}

// CommonDir returns the absolute git common directory, shared by every
// worktree of the project that contains dir.
func CommonDir(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse",
		"--path-format=absolute", "--git-common-dir")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok &&
			exit.ExitCode() == 128 && strings.HasPrefix(
			string(exit.Stderr), "fatal: not a git repository (or any") {
			return "", ErrNotRepo
		}
		return "", fmt.Errorf("not inside a git repository: %w",
			commandError(err))
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// TopLevel returns the root of the worktree containing dir, or "" when
// dir has no worktree: a bare repository, or inside a git directory.
func TopLevel(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		in, inErr := run(dir, "rev-parse", "--is-bare-repository",
			"--is-inside-git-dir")
		if inErr == nil && strings.Contains(in, "true") {
			return "", nil
		}
	}
	return strings.TrimSuffix(out, "\n"), err
}

// CurrentBranch returns the branch checked out in dir, or "" on a
// detached HEAD.
func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "branch", "--show-current")
	return strings.TrimSpace(out), err
}

// ResolveCommit returns the full commit id rev names.
func ResolveCommit(dir, rev string) (string, error) {
	out, err := run(dir, "rev-parse", "--verify", "--quiet",
		rev+"^{commit}")
	if err != nil || out == "" {
		return "", fmt.Errorf("unknown commit %q", rev)
	}
	return strings.TrimSpace(out), nil
}

// Show returns the content of path at commit.
func Show(dir, commit, path string) (string, error) {
	out, err := run(dir, "show", commit+":"+path)
	if err != nil {
		return "", fmt.Errorf("%s not found at %s", path, commit)
	}
	return out, nil
}
