package main_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// diffCase is one testdata/nvim/diff.lua case. path and oldPath are
// relative to the reviewed worktree; the helper resolves the names.
type diffCase struct {
	name     string
	entry    string // The changes buffer line the cursor moves to.
	path     string
	oldPath  string
	old      []string // Lines of the old side.
	new      []string // Lines of the new side.
	fileType string
	deleted  bool
}

// env is the environment of one diff case for the reviewed worktree.
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
		"REVISA_WORKTREE=" + realPath(t, linked),
		"REVISA_COMMIT=" + base,
		"REVISA_OLD_NAME=" + oldName,
		"REVISA_OLD_PATH=" + c.oldPath,
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

// jsonLines renders the buffer lines a case expects as a JSON array.
func jsonLines(t *testing.T, lines []string) string {
	t.Helper()
	data, err := json.Marshal(lines)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// runDiff drives one testdata/nvim/diff.lua case from another worktree.
func runDiff(t *testing.T, p project, linked string, c diffCase) {
	t.Helper()
	headless(t, p.worktrees[0], "diff.lua", c.env(t, linked)...)
}

// committedLines and editedLines are the two versions of src/a.lua in
// the linked worktree of changesProject.
var committedLines = []string{"local a = 1", "local b = 2", "local c = 3"}

var editedLines = []string{"local a = 1", "local x = y", "if x then",
	"  run(x)", "end"}

func TestBufferDiffModifiedFileShowsBothSides(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "modified", entry: "M src/a.lua",
		path: "src/a.lua", oldPath: "src/a.lua",
		old:      committedLines,
		new:      editedLines,
		fileType: "lua",
	})
}

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

func TestBufferDiffRenamedFileShowsOldPath(t *testing.T) {
	p, linked := changesProject(t)
	runDiff(t, p, linked, diffCase{
		name: "renamed", entry: "R src/moved.go -> src/renamed.go",
		path: "src/renamed.go", oldPath: "src/moved.go",
		old: []string{
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
		},
		new: []string{
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
			"line", "line", "line", "line", "line",
			"line",
		},
		fileType: "go",
	})
}

func TestBufferDiffNewSideIsEditable(t *testing.T) {
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

// binaryProject commits a binary file, edits it in the linked worktree
// and adds an untracked binary file there.
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
