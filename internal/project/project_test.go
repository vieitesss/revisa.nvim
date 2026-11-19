package project_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

func TestListWritesNothingToTheStore(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	add(t, p, "src/a.lua", 2, "Check beta")
	add(t, p, "src/a.lua", 1, "Check alpha")
	writeFile(t, linked, "src/a.lua", "gamma\n")
	before := storeFiles(t, p.Store)
	stamp := time.Unix(1_600_000_000, 0)
	for name := range before {
		path := filepath.Join(p.Store, name)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := func(t *testing.T, want string) {
		t.Helper()
		after := storeFiles(t, p.Store)
		if len(after) != len(before) {
			t.Fatalf("store has %d files, had %d",
				len(after), len(before))
		}
		for name, content := range before {
			if after[name] != content {
				t.Errorf("list changed %s", name)
			}
			info, err := os.Stat(filepath.Join(p.Store, name))
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(stamp) {
				t.Errorf("list touched %s: %v", name, info.ModTime())
			}
		}
		for _, status := range []string{"", store.Open} {
			got := list(t, find(t, main), "feat/x", status)
			if len(got) != 2 {
				t.Fatalf("status %q: want 2 comments, got %+v",
					status, got)
			}
			for _, c := range got {
				if c.Anchor.Condition != want {
					t.Errorf("status %q: condition %q, want %q",
						status, c.Anchor.Condition, want)
				}
			}
		}
	}
	unchanged(t, anchor.Outdated)
	gitAt(t, main, "worktree", "remove", "--force", linked)
	gitAt(t, main, "branch", "-q", "-D", "feat/x")
	unchanged(t, anchor.Unknown)
}

func TestNonGitListReanchorsAndMarksOutdated(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.lua", "alpha\nbeta\n")
	p := find(t, root)
	if p.Git {
		t.Fatal("fixture is a git repository")
	}
	add(t, p, "src/a.lua", 2, "Check beta")
	writeFile(t, root, "src/a.lua", "zero\nalpha\nbeta\n")
	got := only(t, list(t, p, "", ""))
	if got.Anchor.Condition != "" || got.Anchor.LineStart != 3 {
		t.Errorf("moved: %+v", got.Anchor)
	}
	if err := os.Remove(filepath.Join(root, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	got = only(t, list(t, p, "", store.Open))
	if got.Anchor.Condition != anchor.Outdated ||
		got.Anchor.LineStart != 2 {
		t.Errorf("gone: %+v", got.Anchor)
	}
	if got.Comment.AnchorText != "beta" {
		t.Errorf("anchor text %q, want %q", got.Comment.AnchorText, "beta")
	}
}

func TestResolveWorksOnAnOutdatedComment(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 2, "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\n")
	if got := only(t, list(t, p, "feat/x", "")); got.Anchor.Condition !=
		anchor.Outdated {
		t.Fatalf("not outdated before resolving: %+v", got.Anchor)
	}

	resolved, err := p.Resolve(c.ID, "Dropped beta.")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Status != store.Resolved || resolved.Reply == nil ||
		*resolved.Reply != "Dropped beta." {
		t.Errorf("resolved: %+v", resolved)
	}
	got := only(t, list(t, p, "feat/x", store.Resolved))
	if got.Anchor.Condition != "" || got.Anchor.LineStart != 2 {
		t.Errorf("resolved comment: %+v", got.Anchor)
	}
	if got.Comment.Reply == nil || *got.Comment.Reply != "Dropped beta." {
		t.Errorf("reply: %+v", got.Comment.Reply)
	}
}
