package main_test

import (
	"path/filepath"
	"testing"
)

func TestEditReplacesBodyOnly(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "2",
		"--body", "Rename x")
	before := listJSON(t, wt)

	out, stderr, code := run(t, wt, "edit", "--body", "Rename x to count", id)
	if code != 0 || out != "edited "+id+"\n" {
		t.Fatalf("exit %d, out %q, stderr %q", code, out, stderr)
	}

	got := listJSON(t, wt)
	if len(got) != 1 || got[0]["body"] != "Rename x to count" {
		t.Fatalf("got %v", got)
	}
	for field, value := range before[0] {
		if field != "body" && got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], value)
		}
	}
}

func TestEditKeepsResolvedReply(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "1", "--body", "b")
	run(t, wt, "resolve", id, "--reply", "done")
	if _, stderr, code := run(t, wt, "edit", id, "--body", "c"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := listJSON(t, wt, "--status", "resolved")
	if len(got) != 1 || got[0]["body"] != "c" || got[0]["reply"] != "done" {
		t.Errorf("got %v", got)
	}
}

func TestEditErrors(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "1", "--body", "b")
	unknown := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	cases := []struct {
		name string
		args []string
		msg  string
		code int
	}{
		{"unknown id", []string{unknown, "--body", "r"},
			`no comment with this id: "` + unknown + `"`, 1},
		{"no id", []string{"--body", "r"}, "pass exactly one comment id", 2},
		{"two ids", []string{id, id, "--body", "r"},
			"pass exactly one comment id", 2},
		{"no body", []string{id}, "--body is required", 2},
		{"blank body", []string{id, "--body", " "}, "--body is required", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"edit"}, c.args...)
			fails(t, wt, c.msg, c.code, args...)
		})
	}
	got := listJSON(t, wt)
	if len(got) != 1 || got[0]["body"] != "b" {
		t.Errorf("comment changed: %v", got)
	}
}

// TestBufferEditComment drives :Revisa edit: the comment on the cursor
// line opens prefilled and saves its new body; off any comment the picker
// lists the file's comments; a buffer with no file target is refused.
func TestBufferEditComment(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "First\nline")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "3-4",
		"--body", "Second")
	headless(t, p.worktrees[0], "edit.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 2 || got[0]["body"] != "First, edited" ||
		got[1]["body"] != "Second" {
		t.Errorf("got %v", got)
	}
}
