package main_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBufferCommentsUseTheDisplayedFileVersion(t *testing.T) {
	p, linked := changesProject(t)
	main := p.worktrees[0]
	base := git(t, linked, "merge-base", "main", "HEAD")
	headless(t, main, "target.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_MAIN_FILE="+filepath.Join(main, "src/a.lua"),
		"REVISA_TARGET="+linked)
	for _, c := range []struct {
		dir  string
		want []map[string]any
	}{
		{main, []map[string]any{{
			"branch": "main", "path": "src/a.lua", "side": "new",
			"anchor_text": "local a = 1", "body": "Main file",
		}}},
		{linked, []map[string]any{
			{
				"branch": "feat/x", "path": "src/a.lua", "side": "new",
				"anchor_text": "local a = 1", "body": "Linked file",
			},
			{
				"branch": "feat/x", "path": "src/gone.go", "side": "old",
				"commit": base, "anchor_text": "package gone",
				"body": "Deleted old file",
			},
		}},
	} {
		got := listJSON(t, c.dir)
		if len(got) != len(c.want) {
			t.Fatalf("from %s: got %v, want %v", c.dir, got, c.want)
		}
		for i, want := range c.want {
			for field, value := range want {
				if got[i][field] != value {
					t.Errorf("%s comment %d %s: got %#v, want %#v",
						c.dir, i, field, got[i][field], value)
				}
			}
		}
	}
}

func TestBufferKeepsASymlinkedFilesPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "real.txt", "first\nsecond\n")
	alias := filepath.Join(root, "alias.txt")
	if err := os.Symlink("real.txt", alias); err != nil {
		t.Fatal(err)
	}
	addOK(t, root, "--path", "alias.txt", "--lines", "1",
		"--body", "Existing alias comment")
	headless(t, root, "symlink.lua", "REVISA_FILE="+alias)
	got := listJSON(t, root, "--file", alias)
	if len(got) != 2 {
		t.Fatalf("want 2 comments on alias.txt, got %v", got)
	}
	for _, c := range got {
		if c["path"] != "alias.txt" {
			t.Errorf("path: got %#v, want alias.txt", c["path"])
		}
	}
}

func TestBufferKeepsDirectorySymlinksLexical(t *testing.T) {
	for _, kind := range []string{"git", "outside git"} {
		for _, link := range []string{"internal", "external"} {
			for _, input := range []string{
				"relative", "absolute", "root alias",
			} {
				t.Run(kind+"/"+link+"/"+input, func(t *testing.T) {
					root := t.TempDir()
					if kind == "git" {
						root = normalClone(t).worktrees[1]
					}
					destination := filepath.Join(root, "src")
					if link == "external" {
						destination = t.TempDir()
					}
					writeFile(t, destination, "real.txt", "first\nsecond\n")
					writeFile(t, destination, "renamed.txt", "renamed\n")
					alias := filepath.Join(root, "alias-dir")
					if err := os.Symlink(destination, alias); err != nil {
						t.Fatal(err)
					}
					path := "alias-dir/real.txt"
					absolute := filepath.Join(realPath(t, root), path)
					if input != "relative" {
						base := root
						if input == "root alias" {
							base = filepath.Join(t.TempDir(), "project")
							if err := os.Symlink(root, base); err != nil {
								t.Fatal(err)
							}
						}
						path = filepath.Join(base, path)
						absolute = path
					}
					addOK(t, root, "--path", "alias-dir/real.txt",
						"--lines", "1", "--body", "Existing alias comment")

					headless(t, root, "directory-symlink.lua",
						"REVISA_FILE="+path, "REVISA_ABSOLUTE="+absolute,
						"REVISA_ROOT="+realPath(t, root))
					got := listJSON(t, root, "--file", absolute)
					if len(got) != 2 {
						t.Fatalf("want 2 alias comments, got %v", got)
					}
					for _, c := range got {
						if c["path"] != "alias-dir/real.txt" {
							t.Errorf("unexpected path: %v", c["path"])
						}
					}
					if link == "external" {
						_, err := os.Stat(filepath.Join(destination, ".revisa"))
						if !os.IsNotExist(err) {
							t.Errorf("outside store: want absent, got %v", err)
						}
					}
				})
			}
		}
	}
}

func TestBufferOldSideKeepsTheReviewedBranch(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	headless(t, p.worktrees[0], "old-branch.lua",
		"REVISA_TARGET="+linked)
	if got := listJSON(t, p.worktrees[0]); len(got) != 0 {
		t.Fatalf("main worktree lists comments: %v", got)
	}
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment on feat/x, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "src/a.lua", "side": "old", "branch": "feat/x",
		"commit": base, "line_start": 2.0, "line_end": 2.0,
		"anchor_text": "local b = 2", "body": "Old side body",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v",
				field, got[0][field], value)
		}
	}
}

func TestBufferReviewKeepsTwoDetachedWorktreesApart(t *testing.T) {
	p, _ := changesProject(t)
	main := p.worktrees[0]
	a := filepath.Join(t.TempDir(), "a", "wt")
	b := filepath.Join(t.TempDir(), "b", "wt")
	git(t, main, "worktree", "add", "-q", "--detach", a, "main")
	git(t, main, "worktree", "add", "-q", "--detach", b, "feat/x")
	names := []string{}
	for _, worktree := range []string{a, b} {
		head := git(t, worktree, "rev-parse", "HEAD")
		names = append(names, "revisa://changes/detached-"+
			head[:7]+"-wt")
	}
	headless(t, main, "detached.lua",
		"REVISA_TARGET_A="+a,
		"REVISA_TARGET_B="+b,
		"REVISA_NAMES="+jsonLines(t, names))
}

func TestBufferRenamedDiffBufferCommentsOnItsNewName(t *testing.T) {
	_, linked := changesProject(t)
	headless(t, linked, "rename.lua",
		"REVISA_LINKED="+realPath(t, linked),
		"REVISA_BASE="+git(t, linked, "merge-base", "main", "HEAD"))
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"path": "renamed.txt", "branch": "feat/x", "side": "new",
		"body": "Comment on the new name",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], value)
		}
	}
}
