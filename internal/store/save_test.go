package store_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/store"
)

func TestSaveRejectsInvalidIDs(t *testing.T) {
	for name, save := range map[string]func(string, store.Comment) error{
		"Save": store.Save, "SaveNew": store.SaveNew,
	} {
		t.Run(name, func(t *testing.T) {
			for _, id := range []string{"", "../invalid",
				"OOOOOOOOOOOOOOOOOOOOOOOOOO"} {
				t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
					dir := filepath.Join(t.TempDir(), "not-created")
					err := save(dir, store.Comment{ID: id})
					want := fmt.Sprintf("invalid comment id %q", id)
					if !errors.Is(err, store.ErrInvalidID) ||
						err.Error() != want {
						t.Errorf("save got %v, want %q", err, want)
					}
					if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
						t.Errorf("invalid id created the store: %v", err)
					}
				})
			}
		})
	}
}

func TestSaveRemovesTemporaryFileOnPublishFailure(t *testing.T) {
	for name, save := range map[string]func(string, store.Comment) error{
		"Save": store.Save, "SaveNew": store.SaveNew,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			c := store.Comment{ID: "01JC8Z7K3M9QX2V5T1R4N6P8WY"}
			destination := filepath.Join(dir, c.ID+".json")
			if err := os.Mkdir(destination, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := save(dir, c); err == nil {
				t.Fatal("save replaced a directory with a comment")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 ||
				entries[0].Name() != c.ID+".json" || !entries[0].IsDir() {
				t.Errorf("failed publication changed store: %v, %v",
					entries, err)
			}
		})
	}
}

func TestSaveReplacesWholeComment(t *testing.T) {
	dir := t.TempDir()
	c := store.Comment{
		ID: "01JC8Z7K3M9QX2V5T1R4N6P8WY", Body: "Original long body",
	}
	if err := store.Save(dir, c); err != nil {
		t.Fatal(err)
	}
	c.Body = "Short"
	if err := store.Save(dir, c); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(dir, c.ID)
	if err != nil || got != c {
		t.Errorf("replacement got %v, %v, want %v", got, err, c)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != c.ID+".json" {
		t.Errorf("temporary files remain: %v, %v", entries, err)
	}
}
