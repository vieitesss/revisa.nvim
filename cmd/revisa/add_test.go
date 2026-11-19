package main_test

import (
	"os"
	"path/filepath"
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

func TestAddErrors(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	file := []string{"--path", "src/a.lua", "--body", "b"}
	cases := []struct {
		name string
		args []string
		msg  string
		code int
	}{
		{"no flags", nil,
			"--path, --lines and --body are required", 2},
		{"blank body", []string{"--path", "src/a.lua",
			"--lines", "1", "--body", "  "},
			"--path, --lines and --body are required", 2},
		{"zero line", append(file, "--lines", "0"),
			"--lines must be N or N-M", 2},
		{"reversed range", append(file, "--lines", "4-2"),
			"--lines must be N or N-M", 2},
		{"not a number", append(file, "--lines", "a"),
			"--lines must be N or N-M", 2},
		{"escaping path", []string{"--path", "../x",
			"--lines", "1", "--body", "b"}, "relative to the repo root", 2},
		{"bad side", append(file, "--lines", "1",
			"--side", "left"), "--side must be new or old", 2},
		{"old without commit", append(file, "--lines", "1",
			"--side", "old"), "--side old requires --commit", 2},
		{"commit on new side", append(file, "--lines", "1",
			"--commit", "HEAD"), "--commit only applies to --side old", 2},
		{"stray argument", append(file, "--lines", "1", "x"),
			`unexpected argument "x"`, 2},
		{"past the end", append(file, "--lines", "5-6"),
			"past the end of src/a.lua (5 lines)", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"add"}, c.args...)
			fails(t, linked, c.msg, c.code, args...)
		})
	}
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("failed adds stored %d comments", len(got))
	}
}

func TestAddMapsAnAbsolutePathToTheProjectRoot(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", filepath.Join(linked, "src/a.lua"),
		"--lines", "2", "--body", "Absolute")
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, want := range map[string]any{
		"path": "src/a.lua", "side": "new", "anchor_text": "local x = y",
		"body": "Absolute",
	} {
		if got[0][field] != want {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], want)
		}
	}
}

func TestAddAndListKeepASymlinkedFilesPath(t *testing.T) {
	for _, kind := range []string{"git", "outside git"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if kind == "git" {
				root = normalClone(t).worktrees[1]
			}
			writeFile(t, root, "real.txt", "anchor\n")
			absolute := filepath.Join(root, "alias.txt")
			if err := os.Symlink("real.txt", absolute); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"alias.txt", absolute} {
				id := addOK(t, root, "--path", path,
					"--lines", "1", "--body", "Alias")
				got := listJSON(t, root, "--file", absolute)
				if len(got) == 0 || got[len(got)-1]["id"] != id {
					t.Fatalf("list --file %q missed %s: %v",
						absolute, id, got)
				}
				for _, c := range got {
					if c["path"] != "alias.txt" {
						t.Errorf("path: got %#v, want alias.txt", c["path"])
					}
				}
			}
		})
	}
}

func TestAddAcceptsAnAbsolutePathToAGoneFileOnTheOldSide(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	if err := os.Remove(filepath.Join(linked, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	id := addOK(t, linked, "--path", filepath.Join(linked, "src/a.lua"),
		"--lines", "2", "--side", "old", "--commit", "HEAD",
		"--body", "Gone from the worktree")
	got := listJSON(t, linked)
	if len(got) != 1 || got[0]["id"] != id {
		t.Fatalf("want the added comment, got %v", got)
	}
	for field, want := range map[string]any{
		"path": "src/a.lua", "side": "old", "anchor_text": "local b = 2",
	} {
		if got[0][field] != want {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], want)
		}
	}
}

func TestAddAndListRefuseAPathOutsideTheProject(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	outside := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, filepath.Dir(outside), "notes.txt", "one\n")
	fails(t, linked, "is outside the project", 1, "add",
		"--path", outside, "--lines", "1", "--body", "b")
	fails(t, linked, "is outside the project", 1, "list", "--file", outside)
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("failed add stored %v", got)
	}
}
