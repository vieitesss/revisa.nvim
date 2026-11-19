package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const placementWrapFile = "first line\nsecond line\nthird line\n" +
	"fourth line\nfifth line\nsixth line\nseventh line\nalpha\nbeta\n"

type storedComment struct {
	id, path, side, anchor, body, commit, status, reply string
	first, last                                         int
}

// storeComment writes one comment file into the project's store, the way
// the CLI reads it back.
func storeComment(t *testing.T, worktree string, comment storedComment) {
	t.Helper()
	common := git(t, worktree,
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	if comment.status == "" {
		comment.status = "open"
	}
	var reply any
	if comment.reply != "" {
		reply = comment.reply
	}
	data, err := json.Marshal(map[string]any{
		"id":          comment.id,
		"branch":      "feat/x",
		"path":        comment.path,
		"side":        comment.side,
		"line_start":  comment.first,
		"line_end":    comment.last,
		"anchor_text": comment.anchor,
		"commit":      comment.commit,
		"body":        comment.body,
		"status":      comment.status,
		"reply":       reply,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(common, "revisa"), comment.id+".json",
		string(data)+"\n")
}

// placementProject builds the files and comments placement.lua draws: one
// linked worktree with a base commit, a head commit, and a comment for each
// case, written straight into the store.
func placementProject(t *testing.T) (p project, base, head string) {
	t.Helper()
	p = normalClone(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "one\ntwo\nthree\nfour\nfive\n")
	git(t, linked, "add", ".")
	git(t, linked, "commit", "-q", "-m", "base")
	base = git(t, linked, "rev-parse", "HEAD")
	writeFile(t, linked, "src/a.lua",
		"local a = 1\nlocal x = y\nif x then\n  run(x)\nend\n")
	git(t, linked, "commit", "-q", "-am", "head")
	head = git(t, linked, "rev-parse", "HEAD")
	writeFile(t, linked, "src/b.lua", "other\n")
	writeFile(t, linked, "src/outdated.lua", "one\ntwo\n")
	writeFile(t, linked, "src/outdated-long.lua", "one\ntwo\nthree\n")
	writeFile(t, linked, "src/wrap.txt", placementWrapFile)
	comments := []storedComment{
		{"01JC8Z7K3M9QX2V5T1R4N6P8WY", "src/a.lua", "new",
			"local x = y\nif x then", "Handle nil", head, "", "", 2, 3},
		{"01JC8Z7K3M9QX2V5T1R4N6P8WZ", "src/a.lua", "new",
			"run(x)\nend", "Resolved body", head, "resolved", "Fixed",
			4, 5},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X0", "src/a.lua", "old", "two",
			"Old side body", base, "", "", 2, 2},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X1", "src/a.lua", "old", "local x = y",
			"Old head body", head, "", "", 2, 2},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X2", "src/b.lua", "new", "other",
			"Other file body", head, "", "", 1, 1},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X3", "src/outdated.lua", "new",
			"not here", "Gone line", head, "", "", 2, 2},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X4", "src/outdated-long.lua", "new",
			"local value = something", "Gone line", head, "", "", 3, 3},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X5", "src/wrap.txt", "new",
			"first line", "before\n\nafter", head, "", "", 1, 1},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X6", "src/wrap.txt", "new",
			"second line", "short", head, "", "", 2, 2},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X7", "src/wrap.txt", "new",
			"third line", "aaaa bbbb cccc dddd eeee", head, "", "", 3, 3},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X8", "src/wrap.txt", "new",
			"fourth line", "    local value = f(a, b)", head, "", "", 4, 4},
		{"01JC8Z7K3M9QX2V5T1R4N6P8X9", "src/wrap.txt", "new",
			"fifth line", "abcdefghijklmnopqrstuvwxyz", head, "", "", 5, 5},
		{"01JC8Z7K3M9QX2V5T1R4N6P8Y0", "src/wrap.txt", "new",
			"sixth line", "one  two   three  four five", head, "", "", 6, 6},
		{"01JC8Z7K3M9QX2V5T1R4N6P8Y1", "src/wrap.txt", "new",
			"seventh line", "ñandú ñandú ñandú ñandú", head, "", "", 7, 7},
		{"01JC8Z7K3M9QX2V5T1R4N6P8Y2", "src/wrap.txt", "new",
			"alpha\nbeta", "first second third fourth fifth", head,
			"resolved", "done here with more details", 8, 9},
	}
	for _, comment := range comments {
		storeComment(t, linked, comment)
	}
	return p, base, head
}

func TestBufferCommentPlacementAndWrapping(t *testing.T) {
	p, base, head := placementProject(t)
	linked := p.worktrees[1]
	headless(t, linked, "placement.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_OUTDATED="+filepath.Join(linked, "src/outdated.lua"),
		"REVISA_OUTDATED_LONG="+
			filepath.Join(linked, "src/outdated-long.lua"),
		"REVISA_WRAP="+filepath.Join(linked, "src/wrap.txt"),
		"REVISA_BASE="+base,
		"REVISA_HEAD="+head)
}

// stubCLI puts a revisa executable that answers every call with payload in
// front of PATH, for a state the CLI reports but a store cannot hold.
func stubCLI(t *testing.T, payload string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "list.json", payload+"\n")
	script := "#!/bin/sh\ncat \"$(dirname \"$0\")/list.json\"\n"
	err := os.WriteFile(filepath.Join(dir, "revisa"), []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBufferUnknownCommentsDrawEndBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "one\ntwo\n")
	stub := stubCLI(t, `[
  {
    "id": "01JC8Z7K3M9QX2V5T1R4N6P8WY",
    "branch": "feat/x",
    "path": "a.txt",
    "side": "new",
    "anchor_text": "one\ntwo",
    "commit": "abc123",
    "body": "The branch is gone",
    "status": "open",
    "reply": null,
    "line_start": null,
    "line_end": null,
    "condition": "unknown"
  },
  {
    "id": "01JC8Z7K3M9QX2V5T1R4N6P8WZ",
    "branch": "feat/x",
    "path": "a.txt",
    "side": "new",
    "anchor_text": "three",
    "commit": "abc123",
    "body": "Gone line",
    "status": "open",
    "reply": null,
    "line_start": null,
    "line_end": null,
    "condition": "outdated"
  }
]`)
	headless(t, root, "unknown.lua",
		"REVISA_FILE="+filepath.Join(root, "a.txt"),
		"PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"))
}
