// Package store reads and writes review comments kept one JSON file each
// in a directory.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// ErrInvalidID reports a comment id that cannot name a stored file.
var ErrInvalidID = errors.New("invalid comment id")

// Delete removes the comment with the given id. A comment another writer
// already deleted is not an error.
func Delete(storeDir, id string) error {
	if !validID(id) {
		return fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	err := os.Remove(filepath.Join(storeDir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Save writes c as <id>.json, replacing it whole so a reader never sees
// a partial file.
func Save(storeDir string, c Comment) error {
	return save(storeDir, c, false)
}

// SaveNew writes c only if its id does not exist. Publication is atomic:
// readers see a whole comment and concurrent creates cannot overwrite it.
// An existing id returns an error wrapping fs.ErrExist.
func SaveNew(storeDir string, c Comment) error {
	return save(storeDir, c, true)
}

func save(storeDir string, c Comment, exclusive bool) error {
	if !validID(c.ID) {
		return fmt.Errorf("%w %q", ErrInvalidID, c.ID)
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
	destination := filepath.Join(storeDir, c.ID+".json")
	if exclusive {
		return os.Link(tmp.Name(), destination)
	}
	return os.Rename(tmp.Name(), destination)
}
