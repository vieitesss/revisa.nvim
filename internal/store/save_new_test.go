package store_test

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/store"
)

func TestSaveNewConcurrentCreatesDoNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	const id = "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	type result struct {
		body string
		err  error
	}
	const writers = 32
	results := make(chan result, writers)
	start := make(chan struct{})
	for i := range writers {
		go func() {
			body := strconv.Itoa(i) + strings.Repeat("x", 64*1024)
			<-start
			err := store.SaveNew(dir, store.Comment{ID: id, Body: body})
			results <- result{body: body, err: err}
		}()
	}
	close(start)
	winner, successes := "", 0
	for range writers {
		r := <-results
		if r.err == nil {
			successes++
			winner = r.body
		} else if !errors.Is(r.err, fs.ErrExist) {
			t.Errorf("unexpected create error: %v", r.err)
		}
		if _, err := store.Load(dir); err != nil {
			t.Errorf("reader saw an incomplete comment: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("%d creates succeeded, want exactly one", successes)
	}
	got, err := store.Get(dir, id)
	if err != nil || got.Body != winner {
		t.Errorf("winning comment was overwritten: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != id+".json" {
		t.Errorf("temporary files remain: %v, %v", entries, err)
	}
}

func TestSaveNewPreservesExistingComment(t *testing.T) {
	dir := t.TempDir()
	original := store.Comment{
		ID: "01JC8Z7K3M9QX2V5T1R4N6P8WY", Body: "Original",
	}
	if err := store.Save(dir, original); err != nil {
		t.Fatal(err)
	}
	replacement := original
	replacement.Body = "Must not replace"
	if err := store.SaveNew(dir, replacement); !errors.Is(err, fs.ErrExist) {
		t.Errorf("SaveNew got %v, want an existing-id error", err)
	}
	got, err := store.Get(dir, original.ID)
	if err != nil || got.Body != "Original" {
		t.Errorf("existing comment changed: %v, %v", got, err)
	}
}
