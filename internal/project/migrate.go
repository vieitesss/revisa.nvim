package project

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/store"
)

// MigrationDir records moves and files left in one local store. Dir and
// Clashes use slash-separated paths relative to Root. Err stops migration;
// Moved still counts successful moves in a directory that later failed.
type MigrationDir struct {
	Dir         string
	Moved       int
	Clashes     []string
	NonComments bool
	Err         error
}

// Migrate moves local comments into the git store on branch. Results follow
// walk order and include the directory in progress if a move fails.
func (p Project) Migrate(branch string) ([]MigrationDir, error) {
	if _, err := os.ReadDir(p.Store); err != nil &&
		!errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var dirs []MigrationDir
	err := filepath.WalkDir(p.Root, func(path string, d fs.DirEntry,
		err error) error {
		if err != nil {
			return err
		}
		if path == p.Root {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			_, err := os.Lstat(filepath.Join(path, ".git"))
			if err == nil {
				return filepath.SkipDir
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		if d.Name() != ".revisa" {
			return nil
		}
		dir, err := p.relativePath(path)
		if err != nil {
			return err
		}
		if !d.IsDir() {
			dirs = append(dirs, MigrationDir{Dir: dir, NonComments: true})
			return nil
		}
		result, err := p.migrateDir(dir, branch)
		result.Err = err
		dirs = append(dirs, result)
		if err != nil {
			return err
		}
		return filepath.SkipDir
	})
	return dirs, err
}

func (p Project) migrateDir(dir, branch string) (MigrationDir, error) {
	result := MigrationDir{Dir: dir}
	path := filepath.Join(p.Root, filepath.FromSlash(dir))
	entries, err := os.ReadDir(path)
	if err != nil {
		return result, err
	}
	prefix := filepath.Dir(dir)
	for _, entry := range entries {
		if !entry.Type().IsRegular() ||
			filepath.Ext(entry.Name()) != ".json" {
			result.NonComments = true
			continue
		}
		source := filepath.Join(path, entry.Name())
		data, err := os.ReadFile(source)
		if err != nil {
			return result, err
		}
		var c store.Comment
		if err := json.Unmarshal(data, &c); err != nil {
			result.NonComments = true
			continue
		}
		c.Branch = branch
		c.Commit = ""
		if prefix != "." {
			c.Path = filepath.ToSlash(filepath.Join(prefix, c.Path))
		}
		if err := store.SaveNew(p.Store, c); err != nil {
			if errors.Is(err, store.ErrInvalidID) {
				result.NonComments = true
				continue
			}
			if !errors.Is(err, fs.ErrExist) {
				return result, err
			}
			file := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			result.Clashes = append(result.Clashes, file)
			continue
		}
		if err := os.Remove(source); err != nil {
			return result, err
		}
		result.Moved++
	}
	if !result.NonComments && len(result.Clashes) == 0 {
		err = os.Remove(path)
	}
	return result, err
}
