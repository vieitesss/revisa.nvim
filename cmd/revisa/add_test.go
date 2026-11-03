package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const committedLua = "local a = 1\nlocal b = 2\nlocal c = 3\n"
const editedLua = "local a = 1\nlocal x = y\nif x then\n  run(x)\nend\n"

// reviewProject is a normal clone whose feat/x worktree committed
// src/a.lua and then edited it without committing.
func reviewProject(t *testing.T) (p project, commit string) {
	t.Helper()
	p = normalClone(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", committedLua)
	git(t, linked, "add", ".")
	git(t, linked, "commit", "-q", "-m", "add a.lua")
	writeFile(t, linked, "src/a.lua", editedLua)
	return p, git(t, linked, "rev-parse", "HEAD")
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func listJSON(t *testing.T, dir string, args ...string) []map[string]any {
	t.Helper()
	out, stderr, code := run(t, dir, append([]string{"list", "--json"},
		args...)...)
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return got
}

func addOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, stderr, code := run(t, dir, append([]string{"add"}, args...)...)
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}
	return strings.TrimSpace(out)
}

func TestAddNewSideReadsAnchorFromWorktree(t *testing.T) {
	p, head := reviewProject(t)
	id := addOK(t, p.worktrees[1],
		"--path", "src/a.lua", "--lines", "2-4", "--body", "Handle nil")

	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %d", len(got))
	}
	want := map[string]any{
		"id":          id,
		"branch":      "feat/x",
		"path":        "src/a.lua",
		"side":        "new",
		"line_start":  2.0,
		"line_end":    4.0,
		"anchor_text": "local x = y\nif x then\n  run(x)",
		"commit":      head,
		"body":        "Handle nil",
		"status":      "open",
		"reply":       nil,
	}
	for field, v := range want {
		if got[0][field] != v {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], v)
		}
	}
	if len(got[0]) != len(want) {
		t.Errorf("got fields %v", got[0])
	}
}

func TestListPrintsShortCommit(t *testing.T) {
	p, head := reviewProject(t)
	id := addOK(t, p.worktrees[1],
		"--path", "src/a.lua", "--lines", "2-4", "--body", "Handle nil")
	out, _, _ := run(t, p.worktrees[1], "list")
	want := id + " src/a.lua:2-4 new " + head[:12] + "\nHandle nil\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestAddForAnotherWorktreesBranch(t *testing.T) {
	p, _ := reviewProject(t)
	addOK(t, p.worktrees[0], "--branch", "feat/x",
		"--path", "src/a.lua", "--lines", "5", "--body", "Why end?")
	got := listJSON(t, p.worktrees[1], "--branch", "feat/x")
	if len(got) != 1 || got[0]["anchor_text"] != "end" {
		t.Errorf("got %v", got)
	}
}

func TestAddForWorktreeWithNewlineInPath(t *testing.T) {
	p := normalClone(t)
	worktree := filepath.Join(t.TempDir(), "line\nbreak")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Skipf("cannot create worktree path containing a newline: %v", err)
	}
	git(t, p.worktrees[0], "worktree", "add", "-q", "-b",
		"feat/newline", worktree)
	writeFile(t, worktree, "file.txt", "anchor\n")
	id := addOK(t, p.worktrees[0], "--branch", "feat/newline",
		"--path", "file.txt", "--lines", "1", "--body", "comment")
	got := listJSON(t, p.worktrees[0], "--branch", "feat/newline")
	if len(got) != 1 || got[0]["id"] != id ||
		got[0]["anchor_text"] != "anchor" {
		t.Errorf("got %v", got)
	}
}

func TestAddOldSideReadsAnchorFromCommit(t *testing.T) {
	p, head := reviewProject(t)
	addOK(t, p.worktrees[1], "--side", "old", "--commit", "feat/x",
		"--path", "src/a.lua", "--lines", "3", "--body", "Why removed?")
	got := listJSON(t, p.worktrees[1])
	if got[0]["anchor_text"] != "local c = 3" || got[0]["commit"] != head {
		t.Errorf("got %v", got[0])
	}
}

func TestAddOldSidePathIsFromRepoRoot(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "a.lua", "root\n")
	writeFile(t, linked, "src/a.lua", "nested\n")
	git(t, linked, "add", ".")
	git(t, linked, "commit", "-q", "-m", "two a.lua")
	addOK(t, filepath.Join(linked, "src"), "--side", "old",
		"--commit", "HEAD", "--path", "./a.lua", "--lines", "1",
		"--body", "b")
	got := listJSON(t, linked)
	if got[0]["path"] != "a.lua" || got[0]["anchor_text"] != "root" {
		t.Errorf("got %v", got[0])
	}
}

func TestAddIDsAreUniqueAndOrdered(t *testing.T) {
	p, _ := reviewProject(t)
	var ids []string
	for range 3 {
		ids = append(ids, addOK(t, p.worktrees[1],
			"--path", "src/a.lua", "--lines", "1", "--body", "x"))
	}
	got := listJSON(t, p.worktrees[1])
	if len(got) != 3 {
		t.Fatalf("want 3 comments, got %d", len(got))
	}
	for i, c := range got {
		if len(ids[i]) != 26 || c["id"] != ids[i] {
			t.Errorf("comment %d: id %v, added %v", i, c["id"], ids)
		}
	}
}

func TestAddErrors(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	detached := filepath.Join(t.TempDir(), "detached")
	git(t, linked, "worktree", "add", "-q", "--detach", detached)
	file := []string{"--path", "src/a.lua", "--body", "b"}
	cases := []struct {
		name string
		dir  string
		args []string
		msg  string
	}{
		{"no flags", linked, nil,
			"--path, --lines and --body are required"},
		{"blank body", linked, []string{"--path", "src/a.lua",
			"--lines", "1", "--body", "  "},
			"--path, --lines and --body are required"},
		{"zero line", linked, append(file, "--lines", "0"),
			"--lines must be N or N-M"},
		{"reversed range", linked, append(file, "--lines", "4-2"),
			"--lines must be N or N-M"},
		{"not a number", linked, append(file, "--lines", "a"),
			"--lines must be N or N-M"},
		{"past the end", linked, append(file, "--lines", "5-6"),
			"past the end of src/a.lua (5 lines)"},
		{"missing file", linked, []string{"--path", "nope.lua",
			"--lines", "1", "--body", "b"}, "nope.lua not found"},
		{"absolute path", linked, []string{"--path", "/etc/hosts",
			"--lines", "1", "--body", "b"}, "relative to the repo root"},
		{"escaping path", linked, []string{"--path", "../x",
			"--lines", "1", "--body", "b"}, "relative to the repo root"},
		{"bad side", linked, append(file, "--lines", "1",
			"--side", "left"), "--side must be new or old"},
		{"old without commit", linked, append(file, "--lines", "1",
			"--side", "old"), "--side old requires --commit"},
		{"commit on new side", linked, append(file, "--lines", "1",
			"--commit", "HEAD"), "--commit only applies to --side old"},
		{"unknown commit", linked, append(file, "--lines", "1",
			"--side", "old", "--commit", "nope"), `unknown commit "nope"`},
		{"path not in commit", linked, []string{"--path", "zz.lua",
			"--lines", "1", "--body", "b", "--side", "old",
			"--commit", "HEAD"}, "zz.lua not found at"},
		{"branch not checked out", linked, append(file, "--lines", "1",
			"--branch", "gone"), `branch "gone" is not checked out`},
		{"detached head", detached, append(file, "--lines", "1"),
			"HEAD is detached, pass --branch"},
		{"stray argument", linked, append(file, "--lines", "1", "x"),
			`unexpected argument "x"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fails(t, c.dir, c.msg, append([]string{"add"}, c.args...)...)
		})
	}
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("failed adds stored %d comments", len(got))
	}
}
