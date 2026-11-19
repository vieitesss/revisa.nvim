package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var revisa string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "revisa-bin")
	if err != nil {
		panic(err)
	}
	revisa = filepath.Join(dir, "revisa")
	build := exec.Command("go", "build", "-o", revisa, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const openOnFeatX = `01JC8Z7K3M9QX2V5T1R4N6P8WY src/a.lua new abc123 outdated
Handle nil here
was: local x = y
was: if x then
was:   run(x)

01JC8Z9A0B1C2D3E4F5G6H7J8K src/b.lua:3 old def456
Why was the cache removed?
Callers still expect it.
`

func TestListResolvedShowsReply(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	out, _, _ := run(t, p.worktrees[1], "list", "--status", "resolved")
	want := `01JC8ZB0000000000000000000 README.md:1-2 new abc123 resolved
Typo in the title
reply: Fixed the title.
`
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestListEmptyStore(t *testing.T) {
	p := normalClone(t)
	out, _, code := run(t, p.worktrees[0], "list")
	if code != 0 || out != "" {
		t.Errorf("exit %d, out %q", code, out)
	}
	out, _, code = run(t, p.worktrees[0], "list", "--json")
	if code != 0 || out != "[]\n" {
		t.Errorf("exit %d, out %q", code, out)
	}
}

func TestUsageExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"no command", nil, "usage: revisa"},
		{"unknown command", []string{"frob"}, `unknown command "frob"`},
		{"bad status", []string{"list", "--status", "bogus"},
			"--status must be open or resolved"},
		{"stray argument", []string{"list", "x"}, `unexpected argument "x"`},
		{"bad boolean", []string{"list", "--json=bogus"},
			`invalid boolean value "bogus" for -json`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fails(t, t.TempDir(), c.msg, 2, c.args...)
		})
	}
}

func TestNonGitMissingStore(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"list"}, {"list", "--json"}} {
		out, stderr, code := run(t, root, args...)
		want := ""
		if len(args) > 1 {
			want = "[]\n"
		}
		if code != 0 || stderr != "" || out != want {
			t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
		}
	}
	id := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	fails(t, root, `no comment with this id: "`+id+`"`, 1,
		"resolve", id, "--reply", "Unknown")
	if _, err := os.Stat(filepath.Join(root, ".revisa")); !os.IsNotExist(err) {
		t.Errorf("read commands created store: %v", err)
	}
}

func TestHelpShowsExitCodes(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			stdout, stderr, code := run(t, t.TempDir(), arg)
			if code != 0 || stderr != "" {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			want := "exit codes:\n" +
				"  0  success\n" +
				"  1  command failed\n" +
				"  2  usage error\n"
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout %q lacks %q", stdout, want)
			}
		})
	}
}

func TestCommandHelpExitsSuccessfully(t *testing.T) {
	for _, command := range []string{
		"list", "add", "resolve", "prune", "migrate",
	} {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(command+"/"+flag, func(t *testing.T) {
				stdout, stderr, code := run(t, t.TempDir(), command, flag)
				want := "Usage of " + command + ":"
				if code != 0 || stdout != "" ||
					!strings.Contains(stderr, want) {
					t.Errorf("exit %d, stdout %q, stderr %q",
						code, stdout, stderr)
				}
			})
		}
	}
}

func TestFlagUsageExitCodes(t *testing.T) {
	cases := []struct {
		command string
		flag    string
	}{
		{"list", "--branch"},
		{"add", "--path"},
		{"resolve", "--reply"},
		{"prune", "--branch"},
		{"migrate", "--branch"},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			t.Run("unknown flag", func(t *testing.T) {
				fails(t, t.TempDir(),
					"flag provided but not defined: -nope",
					2, c.command, "--nope")
			})
			t.Run("missing value", func(t *testing.T) {
				fails(t, t.TempDir(), "flag needs an argument",
					2, c.command, c.flag)
			})
		})
	}
}

func TestCorruptCommentNamesTheFile(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	bad := filepath.Join(p.worktrees[0], ".git", "revisa", "BROKEN.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, p.worktrees[0], "list")
	if code != 1 || !strings.Contains(stderr, "BROKEN.json") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestListReportsUnreadableStore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	store := filepath.Join(p.worktrees[0], ".git", "revisa")
	if err := os.Chmod(store, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(store, 0o755) })
	fails(t, p.worktrees[1], "read store", 1, "list", "--json")
}

func TestStoreIsUntrackedAndSurvivesClean(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[1])
	for _, wt := range p.worktrees {
		if s := git(t, wt, "status", "--porcelain", "--ignored"); s != "" {
			t.Errorf("git status in %s shows %q", wt, s)
		}
		git(t, wt, "clean", "-fdx")
	}
	out, _, _ := run(t, p.worktrees[1], "list")
	if n := strings.Count(out, "01JC8Z"); n != 3 {
		t.Errorf("want 3 comments after git clean, got %d", n)
	}
}
