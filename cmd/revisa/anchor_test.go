package main_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func anchorProject(t *testing.T, content string) (p project, linked string) {
	t.Helper()
	p = normalClone(t)
	linked = p.worktrees[1]
	writeFile(t, linked, "src/a.lua", content)
	git(t, linked, "add", ".")
	git(t, linked, "commit", "-q", "-m", "add src/a.lua")
	return p, linked
}

func TestListReportsMovedLines(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\ngamma\n")
	head := git(t, linked, "rev-parse", "HEAD")
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "zero\nalpha\nbeta\ngamma\n")

	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := id + " src/a.lua:3 new " + head[:12] + "\nCheck beta\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if len(got) != 1 || got[0]["line_start"] != 3.0 ||
		got[0]["line_end"] != 3.0 {
		t.Errorf("json: %v", got)
	}
	if _, ok := got[0]["condition"]; ok {
		t.Errorf("anchored comment has a condition: %v", got[0])
	}
}

func TestListPicksTheMatchNearestTheStoredLine(t *testing.T) {
	for _, c := range []struct {
		name   string
		stored int
		lines  []string
		want   int
	}{
		{"nearest", 5,
			[]string{"hit", "a", "b", "c", "d", "hit"}, 6},
		{"tie goes earlier", 4,
			[]string{"a", "b", "hit", "c", "hit"}, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			initial := make([]string, c.stored)
			for i := range initial {
				initial[i] = "other"
			}
			initial[c.stored-1] = "hit"
			p, linked := anchorProject(t,
				strings.Join(initial, "\n")+"\n")
			addOK(t, linked, "--path", "src/a.lua",
				"--lines", strconv.Itoa(c.stored), "--body", "Hit")
			writeFile(t, linked, "src/a.lua",
				strings.Join(c.lines, "\n")+"\n")
			got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
			if len(got) != 1 || got[0]["line_start"] != float64(c.want) ||
				got[0]["line_end"] != float64(c.want) {
				t.Errorf("want line %d, got %v", c.want, got)
			}
		})
	}
}

func TestListMarksGoneAnchorTextOutdated(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\ngamma\n")
	head := git(t, linked, "rev-parse", "HEAD")
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\ngamma\n")

	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := id + " src/a.lua new " + head[:12] + " outdated\n" +
		"Check beta\nwas: beta\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x",
		"--status", "open")
	if len(got) != 1 || got[0]["condition"] != "outdated" {
		t.Fatalf("json: %v", got)
	}
	for _, field := range []string{"line_start", "line_end"} {
		if v, ok := got[0][field]; !ok || v != nil {
			t.Errorf("%s: got %v, want null", field, v)
		}
	}
}

func TestListAnchorsAnOutdatedCommentAgain(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\ngamma\n")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\ngamma\n")
	if got := listJSON(t, p.worktrees[0],
		"--branch", "feat/x"); got[0]["condition"] != "outdated" {
		t.Fatalf("not outdated: %v", got)
	}
	writeFile(t, linked, "src/a.lua", "new\nalpha\nbeta\ngamma\n")
	out, _, _ := run(t, p.worktrees[0], "list", "--branch", "feat/x")
	if strings.Contains(out, "outdated") || strings.Contains(out, "was:") {
		t.Errorf("still flagged:\n%s", out)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if got[0]["line_start"] != 3.0 || got[0]["line_end"] != 3.0 {
		t.Errorf("json: %v", got)
	}
	if _, ok := got[0]["condition"]; ok {
		t.Errorf("still has a condition: %v", got[0])
	}
}

func TestResolveWorksOnAnOutdatedComment(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\n")
	if got := listJSON(t, p.worktrees[0],
		"--branch", "feat/x"); got[0]["condition"] != "outdated" {
		t.Fatalf("not outdated before resolving: %v", got)
	}
	out, stderr, code := run(t, p.worktrees[0],
		"resolve", id, "--reply", "Dropped beta.")
	if code != 0 || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x",
		"--status", "resolved")
	if len(got) != 1 || got[0]["reply"] != "Dropped beta." ||
		got[0]["line_start"] != 2.0 {
		t.Errorf("resolved comment: %v", got)
	}
	if _, ok := got[0]["condition"]; ok {
		t.Errorf("resolved comment has a condition: %v", got[0])
	}
}

func TestListReadsTheBranchWhenTheWorktreeIsGone(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	writeFile(t, linked, "src/a.lua", "zero\nalpha\nbeta\n")
	git(t, linked, "commit", "-q", "-am", "shift beta")
	head := git(t, linked, "rev-parse", "HEAD")[:12]
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "3",
		"--body", "Check beta")
	writeFile(t, linked, "src/a.lua", "a\nb\nzero\nalpha\nbeta\n")
	git(t, linked, "commit", "-q", "-am", "shift beta again")
	git(t, p.worktrees[0], "worktree", "remove", "--force", linked)

	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := id + " src/a.lua:5 new " + head + "\nCheck beta\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestListMarksAGoneBranchUnknown(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	head := git(t, linked, "rev-parse", "HEAD")[:12]
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta\nAnd lines.")
	git(t, p.worktrees[0], "worktree", "remove", "--force", linked)
	git(t, p.worktrees[0], "branch", "-q", "-D", "feat/x")

	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := id + " src/a.lua new " + head + " unknown\n" +
		"Check beta\nAnd lines.\nwas: beta\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if got[0]["condition"] != "unknown" || got[0]["line_start"] != nil {
		t.Errorf("json: %v", got)
	}
}

func TestListMarksADeletedFileOutdated(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	head := git(t, linked, "rev-parse", "HEAD")[:12]
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	if err := os.Remove(filepath.Join(linked, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	want := id + " src/a.lua new " + head + " outdated\n" +
		"Check beta\nwas: beta\n"
	for _, worktree := range []bool{true, false} {
		if !worktree {
			git(t, linked, "commit", "-q", "-am", "delete a.lua")
			git(t, p.worktrees[0], "worktree", "remove", "--force", linked)
		}
		out, stderr, code := run(t, p.worktrees[0],
			"list", "--branch", "feat/x", "--status", "open")
		if code != 0 {
			t.Fatalf("worktree=%v: exit %d: %s", worktree, code, stderr)
		}
		if out != want {
			t.Errorf("worktree=%v got:\n%s\nwant:\n%s", worktree, out, want)
		}
	}
}

func TestListNeverMarksOldSideOrResolvedCommentsOutdated(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--side", "old", "--commit", "HEAD", "--body", "Old side")
	resolved := addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--body", "Resolved")
	if _, stderr, code := run(t, linked,
		"resolve", resolved, "--reply", "Done."); code != 0 {
		t.Fatal(stderr)
	}
	outdated := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Outdated")
	writeFile(t, linked, "src/a.lua", "nothing left\n")

	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if len(got) != 3 {
		t.Fatalf("want 3 comments, got %v", got)
	}
	for _, c := range got {
		condition, flagged := c["condition"]
		if c["id"] == outdated {
			if !flagged || condition != "outdated" {
				t.Errorf("open comment not outdated: %v", c)
			}
			continue
		}
		if flagged {
			t.Errorf("%v comment has a condition: %v", c["side"], c)
		}
		if c["line_start"] == nil || c["line_end"] == nil {
			t.Errorf("%v comment lost its stored lines: %v", c["side"], c)
		}
	}
	out, _, _ := run(t, p.worktrees[0], "list", "--branch", "feat/x")
	if strings.Count(out, "outdated") != 1 ||
		strings.Count(out, "was:") != 1 {
		t.Errorf("want only the open comment flagged:\n%s", out)
	}
	for _, header := range []string{"src/a.lua:2 old", "src/a.lua:1 new"} {
		if !strings.Contains(out, header) {
			t.Errorf("missing %q:\n%s", header, out)
		}
	}
}

func storeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		files[e.Name()] = readFile(t, filepath.Join(dir, e.Name()))
	}
	return files
}

func TestListWritesNothingToTheStore(t *testing.T) {
	p, linked := anchorProject(t, "alpha\nbeta\n")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--body", "Check alpha")
	writeFile(t, linked, "src/a.lua", "gamma\n")
	dir := filepath.Join(p.worktrees[0], ".git", "revisa")
	stamp := time.Unix(1_600_000_000, 0)
	before := storeFiles(t, dir)
	for name := range before {
		path := filepath.Join(dir, name)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := func(t *testing.T) {
		t.Helper()
		after := storeFiles(t, dir)
		if len(after) != len(before) {
			t.Fatalf("store has %d files, had %d",
				len(after), len(before))
		}
		for name, content := range before {
			if after[name] != content {
				t.Errorf("list changed %s", name)
			}
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(stamp) {
				t.Errorf("list touched %s: %v", name, info.ModTime())
			}
		}
	}
	lists := [][]string{
		{"list", "--branch", "feat/x"},
		{"list", "--branch", "feat/x", "--json"},
		{"list", "--branch", "feat/x", "--status", "open"},
	}
	for _, list := range lists {
		out, stderr, code := run(t, p.worktrees[0], list...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", list, code, stderr)
		}
		if strings.Count(out, "outdated") != 2 {
			t.Errorf("%v: want 2 outdated:\n%s", list, out)
		}
	}
	unchanged(t)
	git(t, p.worktrees[0], "worktree", "remove", "--force", linked)
	git(t, p.worktrees[0], "branch", "-q", "-D", "feat/x")
	for _, list := range lists {
		out, stderr, code := run(t, p.worktrees[0], list...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", list, code, stderr)
		}
		if strings.Count(out, "unknown") != 2 {
			t.Errorf("%v: want 2 unknown:\n%s", list, out)
		}
	}
	unchanged(t)
}

func TestNonGitListReanchorsAndMarksOutdated(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.lua", "alpha\nbeta\n")
	id := addOK(t, root, "--path", "src/a.lua", "--lines", "2",
		"--body", "Check beta")
	writeFile(t, root, "src/a.lua", "zero\nalpha\nbeta\n")
	got := listJSON(t, root)
	if len(got) != 1 || got[0]["line_start"] != 3.0 {
		t.Errorf("moved: %v", got)
	}
	if err := os.Remove(filepath.Join(root, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	out, _, code := run(t, root, "list", "--status", "open")
	if code != 0 ||
		out != id+" src/a.lua new  outdated\nCheck beta\nwas: beta\n" {
		t.Errorf("gone: %q", out)
	}
}

func TestListMarksABlankAnchorInAnEmptiedFileOutdated(t *testing.T) {
	t.Run("emptied file", func(t *testing.T) {
		p, linked := anchorProject(t, "before\n\nafter\n")
		addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
			"--body", "Blank line")
		writeFile(t, linked, "src/a.lua", "")
		got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
		if len(got) != 1 || got[0]["condition"] != "outdated" {
			t.Fatalf("blank anchor is not outdated: %v", got)
		}
		for _, field := range []string{"line_start", "line_end"} {
			if v, ok := got[0][field]; !ok || v != nil {
				t.Errorf("%s: got %v, want null", field, v)
			}
		}
		out, _, code := run(t, p.worktrees[0],
			"list", "--branch", "feat/x", "--status", "open")
		if code != 0 || strings.Count(out, "outdated") != 1 ||
			!strings.Contains(out, "\nwas: \n") {
			t.Errorf("exit %d, want outdated and was: %q", code, out)
		}
	})
	for name, c := range map[string]struct {
		content string
		line    string
		want    float64
	}{
		"blank line inside": {"before\n\nafter\n", "2", 2},
		"blank line at EOF": {"before\nafter\n\n", "3", 3},
		"only a newline":    {"\n", "1", 1},
	} {
		t.Run(name, func(t *testing.T) {
			p, linked := anchorProject(t, c.content)
			addOK(t, linked, "--path", "src/a.lua", "--lines", c.line,
				"--body", "Blank line")
			got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
			if len(got) != 1 {
				t.Fatalf("want 1 comment, got %v", got)
			}
			if _, ok := got[0]["condition"]; ok {
				t.Errorf("blank anchor is %v: %v",
					got[0]["condition"], got[0])
			}
			if got[0]["line_start"] != c.want ||
				got[0]["line_end"] != c.want {
				t.Errorf("got %v, want line %v", got[0], c.want)
			}
		})
	}
}
