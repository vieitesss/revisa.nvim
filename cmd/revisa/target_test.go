package main_test

import (
	"path/filepath"
	"testing"
)

// TestBufferTargetOfEveryBufferKind resolves the reviewing target of a
// file buffer, a changes buffer and all three diff buffers, so the one
// module that answers where a buffer is and which version it shows stays
// one module.
func TestBufferTargetOfEveryBufferKind(t *testing.T) {
	p, linked := changesProject(t)
	main := p.worktrees[0]
	headless(t, main, "target.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_MAIN_FILE="+filepath.Join(main, "src/a.lua"),
		"REVISA_MAIN="+realPath(t, main),
		"REVISA_LINKED="+realPath(t, linked),
		"REVISA_TARGET="+linked,
		"REVISA_HEAD="+git(t, linked, "rev-parse", "HEAD"),
		"REVISA_BASE="+git(t, linked, "merge-base", "main", "HEAD"))
}

// TestBufferOldSideKeepsTheReviewedBranch pins #9: a comment on the old
// side of a review belongs to the branch under review, not to the branch
// the reviewer's own worktree has checked out.
func TestBufferOldSideKeepsTheReviewedBranch(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	headless(t, p.worktrees[0], "old-branch.lua",
		"REVISA_TARGET="+linked)
	if got := listJSON(t, p.worktrees[0]); len(got) != 0 {
		t.Fatalf("main worktree lists comments: %v", got)
	}
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment on feat/x, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/a.lua", "side": "old", "branch": "feat/x",
		"commit": base, "line_start": 2.0, "line_end": 2.0,
		"anchor_text": "local b = 2", "body": "Old side body",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

// TestBufferReviewKeepsTwoDetachedWorktreesApart names a detached changes
// buffer after its head and worktree, so two detached worktrees at
// different heads and an equal basename do not delete each other's buffer.
func TestBufferReviewKeepsTwoDetachedWorktreesApart(t *testing.T) {
	p, _ := changesProject(t)
	main := p.worktrees[0]
	a := filepath.Join(t.TempDir(), "a", "wt")
	b := filepath.Join(t.TempDir(), "b", "wt")
	git(t, main, "worktree", "add", "-q", "--detach", a, "main")
	git(t, main, "worktree", "add", "-q", "--detach", b, "feat/x")
	names := []string{}
	for _, worktree := range []string{a, b} {
		head := git(t, worktree, "rev-parse", "HEAD")
		names = append(names, "revisa://changes/detached-"+
			head[:7]+"-wt")
	}
	headless(t, main, "detached.lua",
		"REVISA_TARGET_A="+a,
		"REVISA_TARGET_B="+b,
		"REVISA_NAMES="+jsonLines(t, names))
}

// TestBufferRenamedDiffBufferCommentsOnItsNewName covers a :saveas on the
// new side of a diff: the pin names the path the diff read, so it stops
// applying once the buffer has another name, and the comment lands on the
// file the reviewer sees.
func TestBufferRenamedDiffBufferCommentsOnItsNewName(t *testing.T) {
	_, linked := changesProject(t)
	headless(t, linked, "rename.lua",
		"REVISA_LINKED="+realPath(t, linked),
		"REVISA_BASE="+git(t, linked, "merge-base", "main", "HEAD"))
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "renamed.txt", "branch": "feat/x", "side": "new",
		"body": "Comment on the new name",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], value)
		}
	}
}
