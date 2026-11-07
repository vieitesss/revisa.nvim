package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBareRepositoryListAndResolve(t *testing.T) {
	p := bareRepo(t)
	addMocks(t, p.worktrees[0])
	bare := git(t, p.worktrees[0],
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	out, stderr, code := run(t, bare,
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 || stderr != "" || out != openOnFeatX {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if got := listJSON(t, bare); len(got) != 0 {
		t.Errorf("main has no comments: %v", got)
	}
	id := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	out, stderr, code = run(t, bare, "resolve", id, "--reply", "Fixed.")
	if code != 0 || stderr != "" || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, bare, "--branch", "feat/x", "--status", "resolved")
	found := false
	for _, c := range got {
		if c["id"] == id && c["reply"] == "Fixed." {
			found = true
		}
	}
	if !found {
		t.Errorf("resolved comment missing: %v", got)
	}
}

func TestBareRepositoryAddKeepsBranchBehavior(t *testing.T) {
	p := bareRepo(t)
	bare := git(t, p.worktrees[0],
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	writeFile(t, p.worktrees[0], "a.lua", "main anchor\n")
	writeFile(t, p.worktrees[1], "a.lua", "feat anchor\n")
	git(t, p.worktrees[0], "add", ".")
	git(t, p.worktrees[0], "commit", "-q", "-m", "add a.lua")
	git(t, bare, "update-ref", "HEAD", "main")
	addOK(t, bare, "--path", "a.lua", "--lines", "1", "--body", "Main")
	got := listJSON(t, bare)
	if len(got) != 1 || got[0]["branch"] != "main" ||
		got[0]["anchor_text"] != "main anchor" {
		t.Errorf("default branch: %v", got)
	}
	addOK(t, bare, "--branch", "feat/x", "--path", "a.lua",
		"--lines", "1", "--body", "Feature")
	got = listJSON(t, bare, "--branch", "feat/x")
	if len(got) != 1 || got[0]["anchor_text"] != "feat anchor" {
		t.Errorf("explicit branch: %v", got)
	}
	addOK(t, bare, "--side", "old", "--commit", "HEAD", "--path",
		"a.lua", "--lines", "1", "--body", "Old side")
	got = listJSON(t, bare)
	if len(got) != 2 {
		t.Fatalf("want 2 comments on main, got %v", got)
	}
	for _, c := range got {
		if c["anchor_text"] != "main anchor" {
			t.Errorf("old side: %v", c)
		}
	}
	fails(t, bare, `branch "gone" is not checked out`, 1, "add",
		"--branch", "gone", "--path", "a.lua", "--lines", "1",
		"--body", "Not checked out")
	git(t, bare, "symbolic-ref", "HEAD", "refs/heads/not-checked-out")
	git(t, bare, "update-ref", "HEAD", "main")
	fails(t, bare, `branch "not-checked-out" is not checked out`, 1,
		"add", "--path", "a.lua", "--lines", "1", "--body", "No worktree")
	git(t, bare, "update-ref", "--no-deref", "HEAD", "main")
	fails(t, bare, "HEAD is detached, pass --branch", 1,
		"add", "--path", "a.lua", "--lines", "1", "--body", "Detached")
}

func TestProjectDiscoveryReportsGitFailures(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"missing git", "run git:"},
		{"invalid gitfile", "invalid gitfile format:"},
		{"missing gitdir", "repository:"},
		{"wrong exit code", "not a git repository (or any"},
		{"invalid repository diagnostic", "not a git repository: /broken"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "a.lua", "anchor\n")
			id := addOK(t, root, "--path", "a.lua", "--lines", "1",
				"--body", "Outside git")
			local := filepath.Join(root, ".revisa", id+".json")
			before, err := os.ReadFile(local)
			if err != nil {
				t.Fatal(err)
			}
			git(t, root, "init", "-q", "-b", "main")
			switch c.name {
			case "missing git":
				t.Setenv("PATH", t.TempDir())
			case "invalid gitfile", "missing gitdir":
				err := os.Rename(filepath.Join(root, ".git"),
					filepath.Join(root, "git-backup"))
				if err != nil {
					t.Fatal(err)
				}
				data := "invalid\n"
				if c.name == "missing gitdir" {
					data = "gitdir: missing\n"
				}
				writeFile(t, root, ".git", data)
			case "wrong exit code", "invalid repository diagnostic":
				bin := t.TempDir()
				diagnostic := "fatal: not a git repository " +
					"(or any of the parent directories): .git"
				exit := "1"
				if c.name == "invalid repository diagnostic" {
					diagnostic = "fatal: not a git repository: /broken"
					exit = "128"
				}
				script := "#!/bin/sh\necho '" + diagnostic +
					"' >&2\nexit " + exit + "\n"
				err := os.WriteFile(filepath.Join(bin, "git"),
					[]byte(script), 0o755)
				if err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin)
			}
			commands := [][]string{
				{"list", "--json"},
				{"add", "--path", "a.lua", "--lines", "1",
					"--body", "Must not save"},
				{"resolve", id, "--reply", "Must not resolve"},
			}
			for _, args := range commands {
				t.Run(args[0], func(t *testing.T) {
					out, stderr, code := run(t, root, args...)
					if code != 1 || out != "" ||
						!strings.Contains(stderr, c.msg) {
						t.Errorf("exit %d, stdout %q, stderr %q",
							code, out, stderr)
					}
					after, err := os.ReadFile(local)
					if err != nil || string(after) != string(before) {
						t.Errorf("local comment changed: %v", err)
					}
					entries, err := os.ReadDir(filepath.Dir(local))
					if err != nil || len(entries) != 1 {
						t.Errorf("local store changed: %v, %v", entries, err)
					}
				})
			}
		})
	}
}

func TestGitProjectIgnoresLocalStore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.lua", "anchor\n")
	localID := addOK(t, root, "--path", "a.lua", "--lines", "1",
		"--body", "Outside git")
	localPath := filepath.Join(root, ".revisa", localID+".json")
	before, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "commit", "-q", "--allow-empty", "-m", "init")
	if got := listJSON(t, root); len(got) != 0 {
		t.Errorf("git project sees local store: %v", got)
	}
	out, stderr, code := run(t, root, "list")
	warning := ".revisa/ comments found in .; run revisa migrate " +
		"to move them into the git store\n"
	if code != 0 || out != "" || stderr != warning {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	out, stderr, code = run(t, root, "add", "--path", "a.lua",
		"--lines", "1", "--body", "Inside git")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	id := strings.TrimSpace(out)
	got := listJSON(t, root)
	if len(got) != 1 || got[0]["id"] != id ||
		got[0]["branch"] != "main" || got[0]["commit"] == "" {
		t.Errorf("git store: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "revisa",
		id+".json")); err != nil {
		t.Fatal(err)
	}
	fails(t, root, "no comment with this id", 1,
		"resolve", localID, "--reply", "Not in git store")
	out, stderr, code = run(t, root, "resolve", id, "--reply", "Fixed.")
	if code != 0 || stderr != "" || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	after, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("git commands changed local comment")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".revisa"))
	if err != nil || len(entries) != 1 {
		t.Errorf("local store changed: %v, %v", entries, err)
	}
}

func TestNonGitNearestAncestorStoreWins(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.lua", "outer\n")
	outerID := addOK(t, root, "--path", "a.lua", "--lines", "1",
		"--body", "Outer project")
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(filepath.Join(inner, ".revisa"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, inner, "a.lua", "inner\n")
	writeFile(t, inner, "deep/a.lua", "deep\n")
	id := addOK(t, filepath.Join(inner, "deep"),
		"--path", "a.lua", "--lines", "1", "--body", "Inner project")
	got := listJSON(t, inner)
	if len(got) != 1 || got[0]["id"] != id ||
		got[0]["path"] != "a.lua" || got[0]["anchor_text"] != "inner" {
		t.Errorf("nearest project: %v", got)
	}
	if got := listJSON(t, root); len(got) != 1 ||
		got[0]["id"] != outerID {
		t.Errorf("outer project changed: %v", got)
	}
}

func TestNonGitListDoesNotFilterByBranch(t *testing.T) {
	root := t.TempDir()
	ids := []string{
		"01JC8Z7K3M9QX2V5T1R4N6P8WY",
		"01JC8ZC0000000000000000000",
	}
	for _, id := range ids {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata",
			"comments", id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, filepath.Join(".revisa", id+".json"),
			string(data))
	}
	got := listJSON(t, root, "--status", "open")
	if len(got) != 2 || got[0]["id"] != ids[0] ||
		got[1]["id"] != ids[1] {
		t.Errorf("comments on different branches: %v", got)
	}
}

func TestNonGitAddAndList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.lua", editedLua)
	id := addOK(t, root, "--path", "src/a.lua", "--lines", "2-4",
		"--body", "Handle nil")

	got := listJSON(t, root)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %d", len(got))
	}
	want := map[string]any{
		"id":          id,
		"branch":      "",
		"path":        "src/a.lua",
		"side":        "new",
		"line_start":  2.0,
		"line_end":    4.0,
		"anchor_text": "local x = y\nif x then\n  run(x)",
		"commit":      "",
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
	path := filepath.Join(root, ".revisa", id+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := run(t, root, "list")
	if code != 0 || stderr != "" ||
		out != id+" src/a.lua:2-4 new \nHandle nil\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
}

func TestNonGitGitOnlyFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"add side old", []string{"add", "--side", "old"}},
		{"add commit", []string{"add", "--commit", "HEAD"}},
		{"add branch", []string{"add", "--branch", "main"}},
		{"list branch", []string{"list", "--branch", "main"}},
		{"add empty commit", []string{"add", "--commit", ""}},
		{"add empty branch", []string{"add", "--branch", ""}},
		{"list empty branch", []string{"list", "--branch", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "a.lua", "anchor\n")
			args := c.args
			if args[0] == "add" {
				args = append(args, "--path", "a.lua", "--lines", "1",
					"--body", "Comment")
			}
			fails(t, root, "needs a git repository", 1, args...)
			_, err := os.Stat(filepath.Join(root, ".revisa"))
			if !os.IsNotExist(err) {
				t.Errorf("failed command created store: %v", err)
			}
		})
	}
}

func TestNonGitCommandsFindAncestorStore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.lua", "root\n")
	first := addOK(t, root, "--path", "a.lua", "--lines", "1",
		"--body", "Root comment")
	writeFile(t, root, "deep/dir/a.lua", "nested\n")
	sub := filepath.Join(root, "deep", "dir")
	id := addOK(t, sub, "--path", "./a.lua", "--lines", "1",
		"--body", "From subdirectory")
	got := listJSON(t, sub)
	if len(got) != 2 {
		t.Fatalf("want 2 comments, got %v", got)
	}
	for _, c := range got {
		if c["path"] != "a.lua" || c["anchor_text"] != "root" {
			t.Errorf("path is not relative to project root: %v", c)
		}
	}
	path := filepath.Join(root, ".revisa", id+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := run(t, sub,
		"resolve", first, "--reply", "Fixed from subdirectory.")
	if code != 0 || stderr != "" || out != "resolved "+first+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	open := listJSON(t, sub, "--status", "open")
	if len(open) != 1 || open[0]["id"] != id {
		t.Errorf("open comments: %v", open)
	}
	resolved := listJSON(t, root, "--status", "resolved")
	if len(resolved) != 1 || resolved[0]["id"] != first ||
		resolved[0]["reply"] != "Fixed from subdirectory." {
		t.Errorf("resolved comments: %v", resolved)
	}
	if got := listJSON(t, root); len(got) != 2 {
		t.Errorf("unfiltered comments: %v", got)
	}
	if _, err := os.Stat(filepath.Join(sub, ".revisa")); !os.IsNotExist(err) {
		t.Errorf("subdirectory store: %v", err)
	}
}

func TestGitMetadataDirListAndResolve(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	meta := filepath.Join(p.worktrees[0], ".git", "objects")
	out, stderr, code := run(t, meta,
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 || stderr != "" || out != openOnFeatX {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	id := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	out, stderr, code = run(t, meta, "resolve", id, "--reply", "Fixed.")
	if code != 0 || stderr != "" || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
}
