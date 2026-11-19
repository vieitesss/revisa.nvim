package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type diffCase struct {
	name     string
	entry    string
	path     string
	oldPath  string
	old      []string
	new      []string
	fileType string
	deleted  bool
}

func (c diffCase) env(t *testing.T, linked string) []string {
	t.Helper()
	base := git(t, linked, "merge-base", "main", "HEAD")
	oldName := "revisa://" + base[:7] + "/" + c.oldPath
	newName := filepath.Join(realPath(t, linked), c.path)
	if c.deleted {
		newName = "revisa://deleted/" + c.path
	}
	return []string{
		"REVISA_CASE=" + c.name,
		"REVISA_TARGET=" + linked,
		"REVISA_ENTRY=" + c.entry,
		"REVISA_OLD_NAME=" + oldName,
		"REVISA_OLD_LINES=" + jsonLines(t, c.old),
		"REVISA_FILE_TYPE=" + c.fileType,
		"REVISA_NEW_NAME=" + newName,
		"REVISA_NEW_LINES=" + jsonLines(t, c.new),
		"REVISA_DELETED=" + boolEnv(c.deleted),
	}
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return ""
}

func jsonLines(t *testing.T, lines []string) string {
	t.Helper()
	data, err := json.Marshal(lines)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runDiff(t *testing.T, p project, linked string, c diffCase) {
	t.Helper()
	headless(t, p.worktrees[0], "diff.lua", c.env(t, linked)...)
}

var committedLines = []string{"local a = 1", "local b = 2", "local c = 3"}

var editedLines = []string{"local a = 1", "local x = y", "if x then",
	"  run(x)", "end"}

func TestBufferDiffAddedFileHasEmptyOldSide(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "added", entry: "A src/new.go",
		path: "src/new.go", oldPath: "src/new.go",
		old:      []string{""},
		new:      []string{"package new"},
		fileType: "go",
	})
}

func TestBufferDiffUntrackedFileHasEmptyOldSide(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "untracked", entry: "? notes.txt",
		path: "notes.txt", oldPath: "notes.txt",
		old: []string{""},
		new: []string{"scratch"},
	})
}

func TestBufferDiffDeletedFileHasEmptyNewSide(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "deleted", entry: "D src/gone.go",
		path: "src/gone.go", oldPath: "src/gone.go",
		old:      []string{"package gone"},
		new:      []string{""},
		fileType: "go",
		deleted:  true,
	})
}

func TestBufferDiffDeletedFileShowsOutdatedBlock(t *testing.T) {
	p, linked := changesProject(t)
	writeFile(t, linked, "src/gone.go", "package gone\n")
	addOK(t, linked, "--path", "src/gone.go", "--lines", "1",
		"--body", "Gone with the file")
	if err := os.Remove(filepath.Join(linked, "src/gone.go")); err != nil {
		t.Fatal(err)
	}
	runDiff(t, p, linked, diffCase{
		name: "deleted-outdated", entry: "D src/gone.go",
		path: "src/gone.go", oldPath: "src/gone.go",
		old:      []string{"package gone"},
		new:      []string{""},
		fileType: "go",
		deleted:  true,
	})
}

func TestBufferDiffOldSideCommentDrawsOnOpen(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--side", "old", "--commit", base, "--body", "Old side body")
	runDiff(t, p, linked, diffCase{
		name: "old-comment", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      committedLines,
		new:      editedLines,
		fileType: "lua",
	})
}

func TestBufferDiffOldSideCommentOnDeletedLine(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	runDiff(t, p, linked, diffCase{
		name: "old-line", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      committedLines,
		new:      editedLines,
		fileType: "lua",
	})
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/a.lua", "side": "old", "branch": "feat/x",
		"commit": base, "line_start": 2.0, "line_end": 2.0,
		"anchor_text": "local b = 2", "body": "Old side body",
		"status": "open",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

func TestBufferDiffRenamedFileOldSideCommentStoresOldPath(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	runDiff(t, p, linked, diffCase{
		name: "old-renamed", entry: "R src/moved.go -> src/renamed.go",
		path: "src/renamed.go", oldPath: "src/moved.go",
		old:      repeatLines("line", 20),
		new:      repeatLines("line", 21),
		fileType: "go",
	})
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/moved.go", "side": "old", "commit": base,
		"line_start": 3.0, "anchor_text": "line",
		"body": "Moved line",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

func repeatLines(text string, n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = text
	}
	return lines
}

func movedBaseProject(t *testing.T) (
	p project, linked, commentCommit string,
) {
	t.Helper()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	git(t, root, "init", "-q", "-b", "main", main)
	writeFile(t, main, "src/a.lua", "local a = 1\nlocal b = 2\n")
	git(t, main, "add", ".")
	git(t, main, "commit", "-q", "-m", "init")
	commentCommit = git(t, main, "rev-parse", "HEAD")
	linked = filepath.Join(root, "linked")
	git(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	writeFile(t, main, "src/a.lua", "local a = 9\nlocal b = 2\n")
	git(t, main, "commit", "-q", "-am", "move")
	git(t, linked, "rebase", "-q", "main")
	writeFile(t, linked, "src/a.lua", "local a = 9\nlocal x = y\n")
	return project{worktrees: []string{main, linked}}, linked, commentCommit
}

func TestBufferDiffOldSideCommentWithMovedBaseIsNotDrawn(t *testing.T) {
	p, linked, commit := movedBaseProject(t)
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--side", "old", "--commit", commit, "--body", "Old side body")
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"side": "old", "commit": commit, "line_start": 2.0,
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
	if condition, flagged := got[0]["condition"]; flagged {
		t.Errorf("old-side comment has a condition: %v", condition)
	}
	out, stderr, code := run(t, linked, "list")
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	if !strings.Contains(out, "old "+commit[:12]) {
		t.Errorf("list does not label side and commit:\n%s", out)
	}
	runDiff(t, p, linked, diffCase{
		name: "old-moved", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      []string{"local a = 9", "local b = 2"},
		new:      []string{"local a = 9", "local x = y"},
		fileType: "lua",
	})
}

func TestBufferDiffNewSideSavesFileAndComment(t *testing.T) {
	p, linked := changesProject(t)
	c := diffCase{
		name: "editable", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      committedLines,
		new:      editedLines,
		fileType: "lua",
	}
	written := []string{"local saved = true", "return saved"}
	env := append(c.env(t, linked),
		"REVISA_WRITE_LINES="+jsonLines(t, written))
	headless(t, p.worktrees[0], "diff.lua", env...)
	want := "local saved = true\nreturn saved\n"
	if got := readFile(t, filepath.Join(linked, "src/a.lua")); got != want {
		t.Errorf("written file %q, want %q", got, want)
	}
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/a.lua", "side": "new", "branch": "feat/x",
		"line_start": 1.0, "line_end": 1.0,
		"anchor_text": "local saved = true", "body": "New side body",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

func TestBufferDiffClosesTabOnQ(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "close", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      committedLines,
		new:      editedLines,
		fileType: "lua",
	})
}

func binaryProject(t *testing.T) (p project, linked string) {
	t.Helper()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	git(t, root, "init", "-q", "-b", "main", main)
	writeFile(t, main, "blob.dat", "\x00binary\n")
	git(t, main, "add", ".")
	git(t, main, "commit", "-q", "-m", "init")
	linked = filepath.Join(root, "linked")
	git(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	writeFile(t, linked, "blob.dat", "changed\x00binary\n")
	writeFile(t, linked, "fresh.dat", "untracked\x00binary\n")
	return project{worktrees: []string{main, linked}}, linked
}

func TestBufferDiffRefusesBinaryFiles(t *testing.T) {
	cases := map[string]diffCase{
		"old side": {name: "binary", entry: "M blob.dat",
			path: "blob.dat", oldPath: "blob.dat"},
		"new side": {name: "binary", entry: "? fresh.dat",
			path: "fresh.dat", oldPath: "fresh.dat"},
	}
	for kind, c := range cases {
		t.Run(kind, func(t *testing.T) {
			p, linked := binaryProject(t)
			env := append(c.env(t, linked),
				"REVISA_BINARY=revisa: "+c.path+" is binary")
			headless(t, p.worktrees[0], "diff.lua", env...)
		})
	}
}
