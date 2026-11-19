package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/project"
)

func TestRelativeMapsAnAbsolutePathInTheWorktree(t *testing.T) {
	_, linked := clone(t)
	writeFile(t, linked, "src/a.lua", "anchor\n")
	got, err := find(t, linked).Relative(filepath.Join(linked, "src/a.lua"))
	if err != nil {
		t.Fatalf("relative: %v", err)
	}
	if got != "src/a.lua" {
		t.Errorf("got %q, want %q", got, "src/a.lua")
	}
}

func TestRelativeMapsAnAbsolutePathOutsideGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".revisa"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "src/a.lua", "anchor\n")
	p := find(t, filepath.Join(root, "src"))
	got, err := p.Relative(filepath.Join(root, "src/a.lua"))
	if err != nil {
		t.Fatalf("relative: %v", err)
	}
	if got != "src/a.lua" {
		t.Errorf("got %q, want %q", got, "src/a.lua")
	}
}

func TestRelativeMapsAPathThatIsGone(t *testing.T) {
	_, linked := clone(t)
	writeFile(t, linked, "src/a.lua", "anchor\n")
	gone := filepath.Join(linked, "src/moved.go")
	got, err := find(t, linked).Relative(gone)
	if err != nil {
		t.Fatalf("relative: %v", err)
	}
	if got != "src/moved.go" {
		t.Errorf("got %q, want %q", got, "src/moved.go")
	}
}

func TestRelativeKeepsARelativePath(t *testing.T) {
	_, linked := clone(t)
	for _, path := range []string{"src/a.lua", "./src/a.lua", "src//a.lua"} {
		got, err := find(t, linked).Relative(path)
		if err != nil {
			t.Fatalf("relative %q: %v", path, err)
		}
		if got != "src/a.lua" {
			t.Errorf("%q: got %q, want %q", path, got, "src/a.lua")
		}
	}
}

func TestRelativeRefusesAPathOutsideTheProject(t *testing.T) {
	_, linked := clone(t)
	outside := filepath.Join(t.TempDir(), "notes.txt")
	_, err := find(t, linked).Relative(outside)
	if err == nil {
		t.Fatal("relative accepted a path outside the project")
	}
	if _, ok := errors.AsType[project.OutsideError](err); !ok {
		t.Fatalf("error %v is not an outside error", err)
	}
	if !strings.Contains(err.Error(), "is outside the project") {
		t.Errorf("error %v does not say it is outside", err)
	}
}
