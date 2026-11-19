package main_test

import (
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
		{"absolute path", []string{"--path", "/etc/hosts",
			"--lines", "1", "--body", "b"}, "relative to the repo root", 2},
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
