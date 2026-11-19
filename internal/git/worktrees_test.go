package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/git"
)

type worktreeList struct {
	list     []git.Worktree
	main     string
	linked   string
	detached string
	bare     string
	symlink  string
}

func newWorktreeList(t *testing.T) worktreeList {
	t.Helper()
	root := t.TempDir()
	w := worktreeList{
		main:     filepath.Join(root, "main"),
		linked:   filepath.Join(root, "linked"),
		detached: filepath.Join(root, "detached"),
		bare:     filepath.Join(root, "repo.git"),
		symlink:  filepath.Join(root, "link"),
	}
	for _, dir := range []string{w.main, w.linked, w.detached, w.bare,
		filepath.Join(w.main, "deep", "dir"),
		filepath.Join(w.linked, "deep")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(w.linked, w.symlink); err != nil {
		t.Fatal(err)
	}
	w.list = []git.Worktree{
		{Path: w.main, Branch: "main", Head: "aaaa"},
		{Path: w.linked, Branch: "feat/x", Head: "bbbb"},
		{Path: w.detached, Head: "bbbb"},
		{Path: w.bare, Branch: "main", Head: "aaaa", Bare: true},
	}
	return w
}

func TestAt(t *testing.T) {
	w := newWorktreeList(t)
	cases := []struct {
		name string
		dir  string
		want string
	}{
		{"root", w.main, w.main},
		{"subdirectory", filepath.Join(w.main, "deep", "dir"), w.main},
		{"symlinked root", w.symlink, w.linked},
		{"symlinked subdirectory", filepath.Join(w.symlink, "deep"), w.linked},
		{"detached", w.detached, w.detached},
		{"bare", w.bare, ""},
		{"outside", t.TempDir(), ""},
		{"missing", filepath.Join(w.main, "gone"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := git.At(w.list, c.dir)
			if c.want == "" {
				if ok {
					t.Fatalf("At(%q) = %q, want none", c.dir, got.Path)
				}
				return
			}
			if !ok || got.Path != c.want {
				t.Errorf("At(%q) = %q, %v, want %q", c.dir, got.Path, ok,
					c.want)
			}
		})
	}
}

func TestNamed(t *testing.T) {
	w := newWorktreeList(t)
	cases := []struct {
		name   string
		path   string
		branch string
		want   string
	}{
		{"path", w.main, "", w.main},
		{"symlinked path", w.symlink, "", w.linked},
		{"branch", "", "feat/x", w.linked},
		{"path before branch", w.main, "feat/x", w.main},
		{"detached by path", w.detached, "", w.detached},
		{"subdirectory is no worktree", filepath.Join(w.linked, "deep"),
			"", ""},
		{"bare path", w.bare, "", ""},
		{"bare branch", "", "main", w.main},
		{"unknown branch", "", "gone", ""},
		{"unknown path", filepath.Join(w.main, "gone"), "", ""},
		{"nothing", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := git.Named(w.list, c.path, c.branch)
			if c.want == "" {
				if ok {
					t.Fatalf("Named(%q, %q) = %q, want none",
						c.path, c.branch, got.Path)
				}
				return
			}
			if !ok || got.Path != c.want {
				t.Errorf("Named(%q, %q) = %q, %v, want %q",
					c.path, c.branch, got.Path, ok, c.want)
			}
		})
	}
}
