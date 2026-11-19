package main_test

import (
	"testing"
)

func TestPrunePrintsCount(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	done := addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--body", "Done")
	if _, stderr, code := run(t, linked, "resolve", done,
		"--reply", "Fixed"); code != 0 {
		t.Fatal(stderr)
	}
	out, stderr, code := run(t, linked, "prune")
	if code != 0 || stderr != "" || out != "pruned 1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	out, _, code = run(t, linked, "prune")
	if code != 0 || out != "pruned 0\n" {
		t.Errorf("second prune exit %d, stdout %q", code, out)
	}
}

func TestPruneErrors(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	fails(t, linked, `unexpected argument "extra"`, 2, "prune", "extra")
	fails(t, t.TempDir(), "needs a git repository", 1,
		"prune", "--branch", "main")
}
