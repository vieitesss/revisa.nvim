package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/git"
	"github.com/vieitesss/revisa.nvim/internal/project"
)

// source reads branch and commit content for one command. Worktrees are
// read once; file content is cached per (commit, path) key.
type source struct {
	dir       string // Where git commands run.
	worktrees []git.Worktree
	content   map[string]string
}

// newSource reads the worktree list of the project that contains dir.
func newSource(dir string) (*source, error) {
	worktrees, err := git.Worktrees(dir)
	if err != nil {
		return nil, err
	}
	return &source{
		dir:       dir,
		worktrees: worktrees,
		content:   map[string]string{},
	}, nil
}

// anchorSource returns the reader of the branch's current files.
func anchorSource(p project.Project) (anchor.Source, error) {
	if !p.Git {
		return dirSource(p.Root), nil
	}
	return newSource(p.Dir())
}

// Read returns the current content of path on branch, from the worktree
// that has branch checked out, else from the branch's tip.
//
// A tip read resolves the commit first, so a branch that is gone is
// anchor.ErrNoBranch, and reports every `git show` failure as
// anchor.ErrNoFile. A commit whose file cannot be read at all therefore
// reads as a missing path; telling the two apart would cost another git
// process per read, so this collapses them on purpose.
func (s *source) Read(branch, path string) (string, error) {
	if wt, ok := s.worktree(branch); ok {
		return readCurrent(wt.Path, path)
	}
	commit, err := git.ResolveCommit(s.dir, "refs/heads/"+branch)
	if err != nil {
		return "", anchor.ErrNoBranch
	}
	content, err := s.show(commit, path)
	if err != nil {
		return "", anchor.ErrNoFile
	}
	return content, nil
}

// ReadNew returns the content of path in the worktree that has branch
// checked out, and that worktree's commit.
func (s *source) ReadNew(branch, path string) (string, string, error) {
	wt, ok := s.worktree(branch)
	if !ok {
		return "", "", fmt.Errorf(
			"branch %q is not checked out in any worktree", branch)
	}
	content, err := readCurrent(wt.Path, path)
	if errors.Is(err, anchor.ErrNoFile) {
		return "", "", fmt.Errorf("%s not found in %s", path, wt.Path)
	}
	if err != nil {
		return "", "", err
	}
	return content, wt.Head, nil
}

// ReadCommit returns the content of path at rev, which may be a short or
// full commit id or a branch, and the full commit it read.
func (s *source) ReadCommit(rev, path string) (string, string, error) {
	commit, err := git.ResolveCommit(s.dir, rev)
	if err != nil {
		return "", "", err
	}
	content, err := s.show(commit, path)
	return content, commit, err
}

// addContent returns the content of the side a comment is made on, and
// the commit it is read at. Outside git there is one side, the file under
// the project root, and no commit.
func addContent(p project.Project, side, branch, rev, path string) (
	content, commit string, err error) {
	if !p.Git {
		content, err = readCurrent(p.Root, path)
		if errors.Is(err, anchor.ErrNoFile) {
			return "", "", fmt.Errorf("%s not found in %s", path, p.Root)
		}
		return content, "", err
	}
	src, err := newSource(p.Dir())
	if err != nil {
		return "", "", err
	}
	if side == "new" {
		return src.ReadNew(branch, path)
	}
	return src.ReadCommit(rev, path)
}

// worktree returns the worktree to read branch from: the invoking one
// when it has the branch, else the first one that has it.
func (s *source) worktree(branch string) (git.Worktree, bool) {
	if wt, ok := git.At(s.worktrees, s.dir); ok && wt.Branch == branch {
		return wt, true
	}
	return git.Named(s.worktrees, "", branch)
}

// show returns path at commit, reading each commit's file once.
func (s *source) show(commit, path string) (string, error) {
	key := commit + "\x00" + path
	if content, ok := s.content[key]; ok {
		return content, nil
	}
	content, err := git.Show(s.dir, commit, path)
	if err != nil {
		return "", err
	}
	s.content[key] = content
	return content, nil
}

// dirSource reads the files under a project root outside git.
type dirSource string

func (d dirSource) Read(_, path string) (string, error) {
	return readCurrent(string(d), path)
}

// readCurrent returns the content of path in the directory dir.
func readCurrent(dir, path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", anchor.ErrNoFile
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
