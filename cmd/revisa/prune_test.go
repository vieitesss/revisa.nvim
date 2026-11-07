package main_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneDeletesResolvedCommentsAndKeepsOpen(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	open := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Still open")
	resolved := addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--body", "Done")
	_, stderr, code := run(t, linked, "resolve", resolved, "--reply", "Fixed")
	if code != 0 {
		t.Fatal(stderr)
	}

	out, stderr, code := run(t, linked, "prune")
	if code != 0 || out != "pruned 1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if got := listJSON(t, linked, "--status", "open"); len(got) != 1 ||
		got[0]["id"] != open {
		t.Errorf("prune changed open comments: %v", got)
	}
	if got := listJSON(t, linked, "--status", "resolved"); len(got) != 0 {
		t.Errorf("resolved comments remain: %v", got)
	}
	out, _, code = run(t, linked, "prune")
	if code != 0 || out != "pruned 0\n" {
		t.Errorf("second prune exit %d, stdout %q", code, out)
	}
}

func TestPruneAllBranchesOrOne(t *testing.T) {
	p, _ := reviewProject(t)
	feat := p.worktrees[1]
	main := p.worktrees[0]
	other := filepath.Join(t.TempDir(), "feat-y")
	git(t, main, "worktree", "add", "-q", "-b", "feat/y", other)
	writeFile(t, other, "src/a.lua", editedLua)
	featID := addOK(t, feat, "--path", "src/a.lua", "--lines", "1",
		"--body", "On feat")
	mainID := addOK(t, main, "--branch", "feat/x", "--path", "src/a.lua",
		"--lines", "1", "--body", "Also on feat")
	otherID := addOK(t, other, "--path", "src/a.lua", "--lines", "1",
		"--body", "On another branch")
	for _, id := range []string{featID, mainID, otherID} {
		_, stderr, code := run(t, main, "resolve", id, "--reply", "Fixed")
		if code != 0 {
			t.Fatal(stderr)
		}
	}

	out, stderr, code := run(t, main, "prune", "--branch", "feat/x")
	if code != 0 || out != "pruned 2\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, main, "--branch", "feat/y", "--status", "resolved")
	if len(got) != 1 || got[0]["id"] != otherID {
		t.Fatalf("prune --branch removed another branch: %v", got)
	}
	if got := listJSON(t, main, "--branch", "feat/x",
		"--status", "resolved"); len(got) != 0 {
		t.Errorf("pruned branch still has resolved comments: %v", got)
	}

	out, stderr, code = run(t, main, "prune")
	if code != 0 || out != "pruned 1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if got := listJSON(t, main, "--branch", "feat/y",
		"--status", "resolved"); len(got) != 0 {
		t.Errorf("prune left resolved comments: %v", got)
	}
}

func TestPruneLocalStore(t *testing.T) {
	root := t.TempDir()
	open := localComment(t, root)
	kept := addOK(t, root, "--path", "src/a.lua", "--lines", "1",
		"--body", "Resolve me")
	_, stderr, code := run(t, root, "resolve", kept, "--reply", "Fixed")
	if code != 0 {
		t.Fatal(stderr)
	}

	out, stderr, code := run(t, root, "prune")
	if code != 0 || out != "pruned 1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, root)
	if len(got) != 1 || got[0]["id"] != open {
		t.Fatalf("local prune changed open comments: %v", got)
	}
	if _, err := os.Stat(
		filepath.Join(root, ".revisa", kept+".json")); !os.IsNotExist(err) {
		t.Errorf("resolved comment still stored: %v", err)
	}
	fails(t, root, "needs a git repository", 1,
		"prune", "--branch", "main")
}

func TestPruneErrors(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	fails(t, linked, `unexpected argument "extra"`, 2, "prune", "extra")
	fails(t, linked, "flag provided but not defined: -nope", 2,
		"prune", "--nope")
}
