package git

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Worktree is one entry of git worktree list.
type Worktree struct {
	Path   string
	Branch string // Empty on a detached HEAD.
	Head   string
	Bare   bool
}

// Worktrees returns every worktree of the project that contains dir,
// bare entries included.
func Worktrees(dir string) ([]Worktree, error) {
	out, err := run(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var worktrees []Worktree
	for record := range strings.SplitSeq(out, "\x00\x00") {
		var wt Worktree
		for field := range strings.SplitSeq(record, "\x00") {
			switch {
			case field == "bare":
				wt.Bare = true
			case strings.HasPrefix(field, "worktree "):
				wt.Path = strings.TrimPrefix(field, "worktree ")
			case strings.HasPrefix(field, "HEAD "):
				wt.Head = strings.TrimPrefix(field, "HEAD ")
			case strings.HasPrefix(field, "branch "):
				wt.Branch = strings.TrimPrefix(strings.TrimPrefix(
					field, "branch "), "refs/heads/")
			}
			// "detached" and "locked" add nothing to an entry.
		}
		if wt.Path != "" {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees, nil
}

// At returns the worktree that holds dir: the one whose path is dir or
// an ancestor of it, both with symlinks resolved. A bare entry holds no
// directory, so it never matches.
func At(list []Worktree, dir string) (Worktree, bool) {
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Worktree{}, false
	}
	for _, wt := range list {
		if wt.Bare {
			continue
		}
		root, err := filepath.EvalSymlinks(wt.Path)
		if err != nil {
			continue
		}
		if holds(root, dir) {
			return wt, true
		}
	}
	return Worktree{}, false
}

// holds reports whether dir is root or inside it.
func holds(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	return err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Named returns the worktree at path, else the first one that has branch
// checked out. An empty path or branch skips that lookup. This is an
// exact path, so a directory inside a worktree does not match.
func Named(list []Worktree, path, branch string) (Worktree, bool) {
	if path != "" {
		for _, wt := range list {
			if !wt.Bare && sameDir(wt.Path, path) {
				return wt, true
			}
		}
	}
	if branch != "" {
		for _, wt := range list {
			if !wt.Bare && wt.Branch == branch {
				return wt, true
			}
		}
	}
	return Worktree{}, false
}

// sameDir reports whether two paths name the same directory, resolving
// symlinks when both exist.
func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}

// DefaultBranch returns the project's default branch, trying the remote
// HEAD, init.defaultBranch, main, then master.
func DefaultBranch(dir string) (string, error) {
	tried := []string{"refs/remotes/origin/HEAD"}
	if name := remoteHead(dir); name != "" && BranchExists(dir, name) {
		return name, nil
	}
	tried = append(tried, "init.defaultBranch")
	if name := configuredDefaultBranch(dir); name != "" &&
		BranchExists(dir, name) {
		return name, nil
	}
	for _, name := range []string{"main", "master"} {
		if BranchExists(dir, name) {
			return name, nil
		}
		tried = append(tried, name)
	}
	return "", fmt.Errorf("no default branch: tried %s",
		strings.Join(tried, ", "))
}

// remoteHead returns the branch refs/remotes/origin/HEAD names, or "".
func remoteHead(dir string) string {
	out, err := run(dir, "symbolic-ref", "--quiet",
		"refs/remotes/origin/HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "refs/remotes/origin/")
}

// configuredDefaultBranch returns init.defaultBranch, or "".
func configuredDefaultBranch(dir string) string {
	out, err := run(dir, "config", "--get", "init.defaultBranch")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// BranchExists reports whether the project has a local branch.
func BranchExists(dir, branch string) bool {
	_, err := run(dir, "rev-parse", "--verify", "--quiet",
		"refs/heads/"+branch)
	return err == nil
}

// MergeBase returns the commit where a and b last shared history.
func MergeBase(dir, a, b string) (string, error) {
	out, err := run(dir, "merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", a, b, err)
	}
	return strings.TrimSpace(out), nil
}

// Change is one session change of a worktree.
type Change struct {
	Status  string // M, A, D, R or ? for an untracked file.
	Path    string
	OldPath string // Only on a rename.
}

// Diff returns what differs between base and the worktree at dir,
// including files git does not track yet.
func Diff(dir, base string) ([]Change, error) {
	out, err := run(dir, "diff", "--name-status", "-M", "-z", base)
	if err != nil {
		return nil, err
	}
	changes := parseNameStatus(out)
	untracked, err := run(dir, "ls-files", "--others",
		"--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for path := range strings.SplitSeq(untracked, "\x00") {
		if path != "" {
			changes = append(changes, Change{Status: "?", Path: path})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes, nil
}

// parseNameStatus reads git diff --name-status -z output: a status then
// one path, or two paths after a rename or copy.
func parseNameStatus(out string) []Change {
	fields := strings.Split(out, "\x00")
	var changes []Change
	for i := 0; i < len(fields); i++ {
		status := fields[i]
		if status == "" {
			continue
		}
		switch letter := status[:1]; letter {
		case "R":
			if i+2 < len(fields) {
				changes = append(changes, Change{
					Status:  "R",
					OldPath: fields[i+1],
					Path:    fields[i+2],
				})
			}
			i += 2
		case "C":
			// A copy reads as a modification of its new path.
			if i+2 < len(fields) {
				changes = append(changes, Change{
					Status: "M", Path: fields[i+2]})
			}
			i += 2
		case "A", "D", "M":
			if i+1 < len(fields) {
				changes = append(changes, Change{
					Status: letter, Path: fields[i+1]})
			}
			i++
		default:
			// A type change reads as a modification.
			if i+1 < len(fields) {
				changes = append(changes, Change{
					Status: "M", Path: fields[i+1]})
			}
			i++
		}
	}
	return changes
}
