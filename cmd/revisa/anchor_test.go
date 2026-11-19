package main_test

import (
	"testing"
)

func TestListShowsAnOutdatedCommentAsJSONAndPlain(t *testing.T) {
	p := normalClone(t)
	linked := p.worktrees[1]
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	head := git(t, linked, "rev-parse", "HEAD")
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\n")

	got := listJSON(t, p.worktrees[0], "--branch", "feat/x",
		"--status", "open")
	if len(got) != 1 || got[0]["condition"] != "outdated" {
		t.Fatalf("json: %v", got)
	}
	for _, field := range []string{"line_start", "line_end"} {
		if v, ok := got[0][field]; !ok || v != nil {
			t.Errorf("%s: got %v, want null", field, v)
		}
	}
	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := id + " src/a.lua new " + head[:12] + " outdated\n" +
		"Check beta\nwas: beta\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

// commitFile writes path under dir and commits it.
func commitFile(t *testing.T, dir, path, content string) {
	t.Helper()
	writeFile(t, dir, path, content)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "write "+path)
}
