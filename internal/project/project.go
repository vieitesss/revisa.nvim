// Package project finds a project's root and comment store.
package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/git"
)

// Project is a git repository or a directory outside git.
type Project struct {
	Store string
	Git   bool
	Root  string // Empty in a bare repository or inside a git directory.
}

// ErrNeedsGit reports a feature unavailable outside git.
var ErrNeedsGit = errors.New("needs a git repository")

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

// Branch returns the current branch, or "" on a detached HEAD.
// Outside git it fails because branches need a git repository.
func (p Project) Branch() (string, error) {
	if !p.Git {
		return "", ErrNeedsGit
	}
	dir := p.Root
	if dir == "" {
		dir = filepath.Dir(p.Store)
	}
	return git.CurrentBranch(dir)
}
