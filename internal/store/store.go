// Package store reads review comments kept one JSON file each in a revisa
// directory under the git common directory, shared by every worktree.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/vieitesss/revisa.nvim/internal/git"
)

// Comment is one reviewer remark, stored as <id>.json.
type Comment struct {
	ID         string  `json:"id"`
	Branch     string  `json:"branch"`
	Path       string  `json:"path"`
	Side       string  `json:"side"`
	LineStart  int     `json:"line_start"`
	LineEnd    int     `json:"line_end"`
	AnchorText string  `json:"anchor_text"`
	Commit     string  `json:"commit"`
	Body       string  `json:"body"`
	Status     string  `json:"status"`
	Reply      *string `json:"reply"`
}

// Dir returns the store directory of the project that contains dir.
func Dir(dir string) (string, error) {
	common, err := git.CommonDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "revisa"), nil
}

// Load reads every comment in the store, ordered by id. A missing store
// holds no comments.
func Load(storeDir string) ([]Comment, error) {
	entries, err := os.ReadDir(storeDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Comment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}
	comments := make([]Comment, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}
		path := filepath.Join(storeDir, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var c Comment
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("read comment %s: %w", path, err)
		}
		comments = append(comments, c)
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].ID < comments[j].ID
	})
	return comments, nil
}

// ErrNotFound reports an id with no comment in the store.
var ErrNotFound = errors.New("no comment with this id")

// Get reads the comment with the given id.
func Get(storeDir, id string) (Comment, error) {
	if !validID(id) {
		return Comment{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	data, err := os.ReadFile(filepath.Join(storeDir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Comment{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err != nil {
		return Comment{}, err
	}
	var c Comment
	if err := json.Unmarshal(data, &c); err != nil {
		return Comment{}, fmt.Errorf("read comment %s: %w", id, err)
	}
	return c, nil
}

// Save writes c as <id>.json, replacing it whole so a reader never sees
// a partial file.
func Save(storeDir string, c Comment) error {
	if !validID(c.ID) {
		return fmt.Errorf("invalid comment id %q", c.ID)
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(storeDir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(storeDir, c.ID+".json"))
}
