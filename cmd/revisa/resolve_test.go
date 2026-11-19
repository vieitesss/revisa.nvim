package main_test

import (
	"testing"
)

func TestResolveErrors(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "1", "--body", "b")
	cases := []struct {
		name string
		args []string
		msg  string
		code int
	}{
		{"no id", []string{"--reply", "r"}, "pass exactly one comment id", 2},
		{"two ids", []string{id, id, "--reply", "r"},
			"pass exactly one comment id", 2},
		{"no reply", []string{id}, "--reply is required", 2},
		{"blank reply", []string{id, "--reply", " "}, "--reply is required", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"resolve"}, c.args...)
			fails(t, wt, c.msg, c.code, args...)
		})
	}
}
