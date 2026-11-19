package main_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

const committedLua = "local a = 1\nlocal b = 2\nlocal c = 3\n"
const editedLua = "local a = 1\nlocal x = y\nif x then\n  run(x)\nend\n"

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

func TestAddNewSidePrefersInvokingWorktreeOfSameBranch(t *testing.T) {
	for _, kind := range []string{"direct", "symlinked parent"} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			first := p.worktrees[1]
			root := t.TempDir()
			if kind == "symlinked parent" {
				link := filepath.Join(t.TempDir(), "linked parent")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			}
			second := filepath.Join(root, "second")
			git(t, first, "worktree", "add", "-q", "--force",
				second, "feat/x")
			writeFile(t, first, "src/a.lua", "one\nFIRST WORKTREE TEXT\n")
			writeFile(t, second, "src/a.lua", "one\nSECOND WORKTREE TEXT\n")
			id := addOK(t, second, "--path", "src/a.lua", "--lines", "2",
				"--body", "Use this worktree")
			got := listJSON(t, second)
			if len(got) != 1 || got[0]["id"] != id ||
				got[0]["anchor_text"] != "SECOND WORKTREE TEXT" {
				t.Errorf("wrong invoking-worktree anchor text: %v", got)
			}
		})
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

func TestAddListsUniqueIDsInLexicalOrder(t *testing.T) {
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
	slices.Sort(ids)
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
		code int
	}{
		{"no flags", linked, nil,
			"--path, --lines and --body are required", 2},
		{"blank body", linked, []string{"--path", "src/a.lua",
			"--lines", "1", "--body", "  "},
			"--path, --lines and --body are required", 2},
		{"zero line", linked, append(file, "--lines", "0"),
			"--lines must be N or N-M", 2},
		{"reversed range", linked, append(file, "--lines", "4-2"),
			"--lines must be N or N-M", 2},
		{"not a number", linked, append(file, "--lines", "a"),
			"--lines must be N or N-M", 2},
		{"past the end", linked, append(file, "--lines", "5-6"),
			"past the end of src/a.lua (5 lines)", 1},
		{"missing file", linked, []string{"--path", "nope.lua",
			"--lines", "1", "--body", "b"}, "nope.lua not found", 1},
		{"absolute path", linked, []string{"--path", "/etc/hosts",
			"--lines", "1", "--body", "b"}, "relative to the repo root", 2},
		{"escaping path", linked, []string{"--path", "../x",
			"--lines", "1", "--body", "b"}, "relative to the repo root", 2},
		{"bad side", linked, append(file, "--lines", "1",
			"--side", "left"), "--side must be new or old", 2},
		{"old without commit", linked, append(file, "--lines", "1",
			"--side", "old"), "--side old requires --commit", 2},
		{"commit on new side", linked, append(file, "--lines", "1",
			"--commit", "HEAD"), "--commit only applies to --side old", 2},
		{"unknown commit", linked, append(file, "--lines", "1",
			"--side", "old", "--commit", "nope"), `unknown commit "nope"`, 1},
		{"path not in commit", linked, []string{"--path", "zz.lua",
			"--lines", "1", "--body", "b", "--side", "old",
			"--commit", "HEAD"}, "zz.lua not found at", 1},
		{"branch not checked out", linked, append(file, "--lines", "1",
			"--branch", "gone"), `branch "gone" is not checked out`, 1},
		{"detached head", detached, append(file, "--lines", "1"),
			"HEAD is detached, pass --branch", 1},
		{"stray argument", linked, append(file, "--lines", "1", "x"),
			`unexpected argument "x"`, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"add"}, c.args...)
			fails(t, c.dir, c.msg, c.code, args...)
		})
	}
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("failed adds stored %d comments", len(got))
	}
}

func TestReposWhosePathsDifferByTrailingSpaceKeepSeparateStores(
	t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	git(t, root, "init", "-q", "-b", "main", seed)
	writeFile(t, seed, "f", "anchor\n")
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-q", "-m", "init")
	var worktrees []string
	for i, name := range []string{"repo.git", "repo.git "} {
		bare := filepath.Join(root, name)
		git(t, root, "clone", "-q", "--bare", seed, bare)
		wt := filepath.Join(root, "wt"+strconv.Itoa(i))
		git(t, bare, "worktree", "add", "-q", wt, "main")
		worktrees = append(worktrees, wt)
	}
	addOK(t, worktrees[1], "--path", "f", "--lines", "1", "--body", "check")
	if got := listJSON(t, worktrees[0]); len(got) != 0 {
		t.Errorf("other repo sees %d comments", len(got))
	}
	if got := listJSON(t, worktrees[1]); len(got) != 1 {
		t.Errorf("want 1 comment, got %d", len(got))
	}
}

func TestAddOldSideOnDeletedLine(t *testing.T) {
	_, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	addOK(t, linked, "--side", "old", "--commit", base,
		"--path", "src/gone.go", "--lines", "1", "--body", "Why gone?")
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/gone.go", "side": "old", "branch": "feat/x",
		"commit": base, "anchor_text": "package gone",
		"body": "Why gone?",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

func TestAddOldSideOnRenamedFileStoresOldPath(t *testing.T) {
	_, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	addOK(t, linked, "--side", "old", "--commit", base,
		"--path", "src/moved.go", "--lines", "3", "--body", "Moved why?")
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/moved.go", "side": "old", "commit": base,
		"anchor_text": "line",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}
