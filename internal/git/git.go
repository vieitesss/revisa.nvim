// Package git runs the git commands revisa needs, reporting git's own
// message when one fails.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", errors.New(strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("run git: %w", err)
	}
	return string(out), nil
}

// CommonDir returns the absolute git common directory, shared by every
// worktree of the project that contains dir.
func CommonDir(dir string) (string, error) {
	out, err := run(dir,
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimSuffix(out, "\n"), nil
}

// CurrentBranch returns the branch checked out in dir, or "" on a
// detached HEAD.
func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "branch", "--show-current")
	return strings.TrimSpace(out), err
}

// Head returns the full commit id of HEAD in dir.
func Head(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "HEAD")
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

// WorktreeOf returns the root of the worktree where branch is checked
// out, or "" when no worktree has it.
func WorktreeOf(dir, branch string) (string, error) {
	out, err := run(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", err
	}
	var path string
	for field := range strings.SplitSeq(out, "\x00") {
		if p, ok := strings.CutPrefix(field, "worktree "); ok {
			path = p
		}
		if field == "branch refs/heads/"+branch {
			return path, nil
		}
	}
	return "", nil
}
