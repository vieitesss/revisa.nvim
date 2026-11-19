package main_test

import (
	"testing"
)

func TestResolveStoresReply(t *testing.T) {
	p, _ := reviewProject(t)
	id := addOK(t, p.worktrees[0], "--branch", "feat/x",
		"--path", "src/a.lua", "--lines", "2", "--body", "Rename x")

	out, stderr, code := run(t, p.worktrees[1],
		"resolve", "--reply", "Renamed to count.", id)
	if code != 0 || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, out %q, stderr %q", code, out, stderr)
	}

	if open := listJSON(t, p.worktrees[1], "--status", "open"); len(open) != 0 {
		t.Errorf("still open: %v", open)
	}
	got := listJSON(t, p.worktrees[1], "--status", "resolved")
	if len(got) != 1 || got[0]["reply"] != "Renamed to count." {
		t.Errorf("got %v", got)
	}
}

func TestResolveErrors(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "1", "--body", "b")
	resolved := addOK(t, wt,
		"--path", "src/a.lua", "--lines", "1", "--body", "b")
	run(t, wt, "resolve", resolved, "--reply", "first")

	unknown := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	cases := []struct {
		name string
		args []string
		msg  string
		code int
	}{
		{"unknown id", []string{unknown, "--reply", "r"},
			`no comment with this id: "` + unknown + `"`, 1},
		{"path as id", []string{"../../HEAD", "--reply", "r"},
			"no comment with this id", 1},
		{"no id", []string{"--reply", "r"}, "pass exactly one comment id", 2},
		{"two ids", []string{id, id, "--reply", "r"},
			"pass exactly one comment id", 2},
		{"no reply", []string{id}, "--reply is required", 2},
		{"blank reply", []string{id, "--reply", " "}, "--reply is required", 2},
		{"already resolved", []string{resolved, "--reply", "again"},
			"already resolved", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"resolve"}, c.args...)
			fails(t, wt, c.msg, c.code, args...)
		})
	}
	got := listJSON(t, wt, "--status", "resolved")
	if len(got) != 1 || got[0]["reply"] != "first" {
		t.Errorf("resolved comment changed: %v", got)
	}
}
