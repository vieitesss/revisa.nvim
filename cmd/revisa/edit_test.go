package main_test

import (
	"path/filepath"
	"testing"
)

func TestBufferEditRefusesUnsavedSource(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Original")
	headless(t, p.worktrees[0], "edit-unsaved.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 1 || got[0]["body"] != "Original" {
		t.Errorf("got %v", got)
	}
}

func TestBufferEditRefusesModifiedSource(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Original")
	headless(t, p.worktrees[0], "edit-modified.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 1 || got[0]["body"] != "Original" {
		t.Errorf("got %v", got)
	}
}

func TestBufferEditStopsWhenRefreshFails(t *testing.T) {
	cases := []struct {
		name  string
		count int
	}{
		{"no comments", 0},
		{"one comment", 1},
		{"several comments", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			for range c.count {
				addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
					"--body", "Original")
			}
			headless(t, p.worktrees[0], "edit-refresh-failure.lua",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_WORKTREE="+linked)
			git(t, linked, "checkout", "-q", "feat/x")
			got := listJSON(t, linked)
			if len(got) != c.count {
				t.Fatalf("got %v", got)
			}
			for _, comment := range got {
				if comment["body"] != "Original" {
					t.Errorf("comment changed: %v", comment)
				}
			}
		})
	}
}

func TestBufferEditPicksTheDrawnComment(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "First\nline")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "3-4",
		"--body", "Second")
	headless(t, p.worktrees[0], "edit-moved.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 2 || got[0]["body"] != "First\nline" ||
		got[1]["body"] != "Second" {
		t.Errorf("got %v", got)
	}
}

func TestEditPrintsTheCommentID(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "2", "--body", "b")
	out, stderr, code := run(t, wt, "edit", "--body", "c", id)
	if code != 0 || stderr != "" || out != "edited "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
}

func TestEditErrors(t *testing.T) {
	p, _ := reviewProject(t)
	wt := p.worktrees[1]
	id := addOK(t, wt, "--path", "src/a.lua", "--lines", "1", "--body", "b")
	cases := []struct {
		name string
		args []string
		msg  string
		code int
	}{
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
	if got := listJSON(t, wt); len(got) != 1 || got[0]["body"] != "b" {
		t.Errorf("comment changed: %v", got)
	}
}

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
