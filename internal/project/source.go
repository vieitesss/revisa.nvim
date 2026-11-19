package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/git"
)

type source struct {
	dir       string
	worktrees []git.Worktree
	content   map[string]string
}

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

func anchorSource(p Project) (anchor.Source, error) {
	if !p.Git {
		return dirSource(p.Root), nil
	}
	return newSource(p.Dir())
}

// Read reports every git show failure as anchor.ErrNoFile, without a
// separate probe to distinguish missing paths from other read failures.
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

func (s *source) ReadCommit(rev, path string) (string, string, error) {
	commit, err := git.ResolveCommit(s.dir, rev)
	if err != nil {
		return "", "", err
	}
	content, err := s.show(commit, path)
	return content, commit, err
}

func addContent(p Project, side, branch, rev, path string) (
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

func (s *source) worktree(branch string) (git.Worktree, bool) {
	if wt, ok := git.At(s.worktrees, s.dir); ok && wt.Branch == branch {
		return wt, true
	}
	return git.Named(s.worktrees, "", branch)
}

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

type dirSource string

func (d dirSource) Read(_, path string) (string, error) {
	return readCurrent(string(d), path)
}

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
