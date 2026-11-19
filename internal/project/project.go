// Package project manages a project's comments and locates its store.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/git"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

// Project is a git repository or a directory outside git.
type Project struct {
	Store string
	Git   bool
	Root  string // Empty in a bare repository or inside a git directory.
}

// ErrNeedsGit reports a feature unavailable outside git.
var ErrNeedsGit = errors.New("needs a git repository")

// OutsideError reports an absolute path outside the project.
type OutsideError struct {
	Path string
	Root string
}

func (e OutsideError) Error() string {
	return fmt.Sprintf("%s is outside the project %s", e.Path, e.Root)
}

// Find locates the project containing wd without creating its store.
func Find(wd string) (Project, error) {
	root, err := filepath.Abs(wd)
	if err != nil {
		return Project{}, err
	}
	common, err := git.CommonDir(root)
	if err == nil {
		root, err = git.TopLevel(root)
		if err != nil {
			return Project{}, err
		}
		return Project{
			Store: filepath.Join(common, "revisa"),
			Git:   true,
			Root:  root,
		}, nil
	}
	if !errors.Is(err, git.ErrNotRepo) {
		return Project{}, err
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(filepath.Join(dir, ".revisa"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Project{}, err
		}
		if err == nil && info.IsDir() {
			root = dir
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return Project{
		Store: filepath.Join(root, ".revisa"),
		Root:  root,
	}, nil
}

// Dir returns where the project keeps its files and where its git
// commands run: the worktree root, or the git common directory in a bare
// repository.
func (p Project) Dir() string {
	if p.Root != "" {
		return p.Root
	}
	return filepath.Dir(p.Store)
}

// Relative maps path to the one the store keeps, relative to the project
// root, with slash separators. A relative path is taken as already
// relative; an absolute one must be inside the root. Only aliases of the
// root and its ancestors are resolved; the suffix stays lexical, even
// through in-project symlinks. The file need not exist, so an old-side
// path survives a rename.
func (p Project) Relative(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path)), nil
	}
	root := p.Dir()
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	separator := string(filepath.Separator)
	dir := filepath.VolumeName(path) + separator
	rest := strings.TrimPrefix(filepath.Clean(path), dir)
	// Stop at the first spelling of the root, before resolving any of
	// the suffix: a symlink inside the project is part of its identity.
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			break
		}
		if real == realRoot {
			return filepath.ToSlash(filepath.Clean(rest)), nil
		}
		if rest == "" {
			break
		}
		next, tail, _ := strings.Cut(rest, separator)
		dir, rest = filepath.Join(dir, next), tail
	}
	return "", OutsideError{Path: path, Root: root}
}

// Branch returns the current branch, or "" on a detached HEAD.
// Outside git it fails because branches need a git repository.
func (p Project) Branch() (string, error) {
	if !p.Git {
		return "", ErrNeedsGit
	}
	return git.CurrentBranch(p.Dir())
}

// CommentBranch defaults branch to the current branch in git, refusing a
// detached HEAD. Outside git it leaves branch unchanged.
func (p Project) CommentBranch(command, branch string) (string, error) {
	if !p.Git || branch != "" {
		return branch, nil
	}
	branch, err := p.Branch()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	if branch == "" {
		return "", fmt.Errorf("%s: HEAD is detached, pass --branch", command)
	}
	return branch, nil
}

// Draft describes a comment before its ID and anchor text are captured.
// Start and End are one-based, inclusive line numbers.
type Draft struct {
	Path   string
	Start  int
	End    int
	Body   string
	Side   string
	Branch string
	Commit string
}

// LinesPastEndError reports a range beyond a file's current content.
type LinesPastEndError struct {
	Path      string
	LineCount int
}

func (e LinesPastEndError) Error() string {
	return fmt.Sprintf("add: lines are past the end of %s (%d lines)",
		e.Path, e.LineCount)
}

// Add captures draft's anchor text from its file version and saves a new
// open comment. An empty branch defaults to the current branch in git.
func (p Project) Add(draft Draft) (store.Comment, error) {
	if !p.Git && draft.Side == "old" {
		return store.Comment{}, fmt.Errorf("add: --side old %w", ErrNeedsGit)
	}
	branch, err := p.CommentBranch("add", draft.Branch)
	if err != nil {
		return store.Comment{}, err
	}
	content, commit, err := addContent(p, draft.Side, branch, draft.Commit,
		draft.Path)
	if err != nil {
		return store.Comment{}, fmt.Errorf("add: %w", err)
	}
	fileLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" || draft.End > len(fileLines) {
		return store.Comment{}, LinesPastEndError{
			Path: draft.Path, LineCount: len(fileLines),
		}
	}
	c := store.Comment{
		ID:         store.NewID(time.Now()),
		Branch:     branch,
		Path:       draft.Path,
		Side:       draft.Side,
		LineStart:  draft.Start,
		LineEnd:    draft.End,
		AnchorText: strings.Join(fileLines[draft.Start-1:draft.End], "\n"),
		Commit:     commit,
		Body:       draft.Body,
		Status:     store.Open,
	}
	if err := store.SaveNew(p.Store, c); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return store.Comment{}, fmt.Errorf(
				"add: comment %s already exists", c.ID)
		}
		return store.Comment{}, err
	}
	return c, nil
}

// Edit replaces a comment's body without changing its status or anchor.
func (p Project) Edit(id, body string) (store.Comment, error) {
	c, err := store.Get(p.Store, id)
	if err != nil {
		return store.Comment{}, fmt.Errorf("edit: %w", err)
	}
	c.Body = body
	if err := store.Save(p.Store, c); err != nil {
		return store.Comment{}, err
	}
	return c, nil
}

// Resolve records a reply and resolves a comment that is not yet resolved.
func (p Project) Resolve(id, reply string) (store.Comment, error) {
	c, err := store.Get(p.Store, id)
	if err != nil {
		return store.Comment{}, fmt.Errorf("resolve: %w", err)
	}
	if c.Status == store.Resolved {
		return store.Comment{}, fmt.Errorf(
			"resolve: comment %s is already resolved", c.ID)
	}
	c.Status = store.Resolved
	c.Reply = &reply
	if err := store.Save(p.Store, c); err != nil {
		return store.Comment{}, err
	}
	return c, nil
}

// LocatedComment pairs a stored comment with its current anchor location.
type LocatedComment struct {
	store.Comment
	Anchor anchor.Result
}

// List locates comments on branch, optionally filtered by status and by
// one file path. An empty branch defaults to the current branch in git;
// outside git it is ignored. An empty path lists every file.
func (p Project) List(branch, status, path string) ([]LocatedComment, error) {
	branch, err := p.CommentBranch("list", branch)
	if err != nil {
		return nil, err
	}
	all, err := store.Load(p.Store)
	if err != nil {
		return nil, err
	}
	src, err := anchorSource(p)
	if err != nil {
		return nil, err
	}
	matched := []LocatedComment{}
	for _, c := range all {
		if p.Git && c.Branch != branch {
			continue
		}
		if status != "" && c.Status != status {
			continue
		}
		if path != "" && c.Path != path {
			continue
		}
		located, err := anchor.Locate(c, src)
		if err != nil {
			return nil, err
		}
		matched = append(matched, LocatedComment{Comment: c, Anchor: located})
	}
	return matched, nil
}

// Prune deletes resolved comments on branch, or every branch when empty.
// It does not require a current branch, so detached HEADs are allowed.
func (p Project) Prune(branch string) (int, error) {
	all, err := store.Load(p.Store)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, c := range all {
		if c.Status != store.Resolved {
			continue
		}
		if branch != "" && c.Branch != branch {
			continue
		}
		if err := store.Delete(p.Store, c.ID); err != nil {
			return pruned, fmt.Errorf("prune: %w", err)
		}
		pruned++
	}
	return pruned, nil
}
