package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

func TestListReadsTheBranchWhenTheWorktreeIsGone(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	writeFile(t, linked, "src/a.lua", "zero\nalpha\nbeta\n")
	gitAt(t, linked, "commit", "-q", "-am", "shift beta")
	head := gitAt(t, linked, "rev-parse", "HEAD")
	c := add(t, find(t, linked), "src/a.lua", 3, "Check beta")
	writeFile(t, linked, "src/a.lua", "a\nb\nzero\nalpha\nbeta\n")
	gitAt(t, linked, "commit", "-q", "-am", "shift beta again")
	gitAt(t, main, "worktree", "remove", "--force", linked)

	got := only(t, list(t, find(t, main), "feat/x", ""))
	if got.Anchor.Condition != "" || got.Anchor.LineStart != 5 ||
		got.Anchor.LineEnd != 5 {
		t.Errorf("anchor: %+v", got.Anchor)
	}
	if got.Comment.Commit != head {
		t.Errorf("commit %q, want %q", got.Comment.Commit, head)
	}
	if got.Comment.ID != c.ID {
		t.Errorf("id %q, want %q", got.Comment.ID, c.ID)
	}
}

func TestListMarksAGoneBranchUnknown(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	add(t, find(t, linked), "src/a.lua", 2, "Check beta\nAnd lines.")
	gitAt(t, main, "worktree", "remove", "--force", linked)
	gitAt(t, main, "branch", "-q", "-D", "feat/x")

	got := only(t, list(t, find(t, main), "feat/x", store.Open))
	if got.Anchor.Condition != anchor.Unknown {
		t.Fatalf("condition %q, want %q",
			got.Anchor.Condition, anchor.Unknown)
	}
	if got.Anchor.LineStart != 2 || got.Anchor.LineEnd != 2 {
		t.Errorf("lines: %+v", got.Anchor)
	}
	if got.Comment.AnchorText != "beta" {
		t.Errorf("anchor text %q, want %q", got.Comment.AnchorText, "beta")
	}
}

func TestListMarksADeletedFileOutdated(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	add(t, find(t, linked), "src/a.lua", 2, "Check beta")
	if err := os.Remove(filepath.Join(linked, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	for _, worktree := range []bool{true, false} {
		if !worktree {
			gitAt(t, linked, "commit", "-q", "-am", "delete a.lua")
			gitAt(t, main, "worktree", "remove", "--force", linked)
		}
		got := only(t, list(t, find(t, main), "feat/x", store.Open))
		if got.Anchor.Condition != anchor.Outdated {
			t.Errorf("worktree=%v: condition %q, want %q", worktree,
				got.Anchor.Condition, anchor.Outdated)
		}
		if got.Comment.AnchorText != "beta" {
			t.Errorf("worktree=%v: anchor text %q", worktree,
				got.Comment.AnchorText)
		}
	}
}
