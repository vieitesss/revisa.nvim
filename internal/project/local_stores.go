package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// PendingLocalStore identifies an ancestor store with JSON entries or a
// read error. Dir is its containing directory, relative to Root, with slash
// separators. Err is non-nil if the local store could not be checked.
type PendingLocalStore struct {
	Dir string
	Err error
}

// PendingLocalStores checks wd's ancestors through Root, nearest first.
// It does not parse or move comments, follow local-store symlinks, or inspect
// non-git projects and bare repositories. Per-store errors remain in results
// so one unreadable local store does not prevent checking other ancestors.
func (p Project) PendingLocalStores(wd string) ([]PendingLocalStore, error) {
	if !p.Git || p.Root == "" {
		return nil, nil
	}
	wd, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return nil, err
	}
	var pending []PendingLocalStore
	for dir := wd; ; dir = filepath.Dir(dir) {
		rel, err := p.relativePath(dir)
		if err != nil {
			return pending, err
		}
		local := filepath.Join(dir, ".revisa")
		info, err := os.Lstat(local)
		if err == nil && info.IsDir() {
			var entries []os.DirEntry
			entries, err = os.ReadDir(local)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() &&
						filepath.Ext(entry.Name()) == ".json" {
						pending = append(pending, PendingLocalStore{Dir: rel})
						break
					}
				}
			}
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			pending = append(pending, PendingLocalStore{Dir: rel, Err: err})
		}
		if dir == p.Root || filepath.Dir(dir) == dir {
			break
		}
	}
	return pending, nil
}

func (p Project) relativePath(path string) (string, error) {
	rel, err := filepath.Rel(p.Root, path)
	return filepath.ToSlash(rel), err
}
