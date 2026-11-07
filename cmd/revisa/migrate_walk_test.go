package main_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateWalksOnlyCurrentWorktree(t *testing.T) {
	outer := t.TempDir()
	outerID := localComment(t, outer)
	root := filepath.Join(outer, "repository")
	rootID := localComment(t, root)
	second := localComment(t, root)
	sub := filepath.Join(root, "sub", "deep")
	nestedID := localComment(t, sub)
	sibling := filepath.Join(root, "sibling")
	siblingID := localComment(t, sibling)
	inner := filepath.Join(root, "other-project")
	innerID := localComment(t, inner)
	initProject(t, inner)
	initProject(t, root)
	gitComment := filepath.Join(root, ".git", "private", ".revisa")
	writeFile(t, gitComment, "ignored.json", "{")

	out, stderr, code := run(t, sub, "migrate")
	want := "moved 2 comments from .revisa/\n" +
		"moved 1 comment from sibling/.revisa/\n" +
		"moved 1 comment from sub/deep/.revisa/\n"
	if code != 0 || stderr != "" || out != want {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, root)
	wantPaths := map[string]string{
		rootID: "src/a.lua", second: "src/a.lua",
		nestedID: "sub/deep/src/a.lua", siblingID: "sibling/src/a.lua",
	}
	if len(got) != len(wantPaths) {
		t.Fatalf("migrated comments: %v", got)
	}
	for _, c := range got {
		if c["path"] != wantPaths[c["id"].(string)] ||
			c["branch"] != "main" || c["commit"] != "" {
			t.Errorf("moved comment fields: %v", c)
		}
	}
	for _, dir := range []string{root, sub, sibling} {
		path := filepath.Join(dir, ".revisa")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("migrated local store remains in %s: %v", dir, err)
		}
	}
	for dir, id := range map[string]string{outer: outerID, inner: innerID} {
		path := filepath.Join(dir, ".revisa", id+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("comment outside this project changed: %v", err)
		}
	}
	gitFile := filepath.Join(gitComment, "ignored.json")
	if got := readFile(t, gitFile); got != "{" {
		t.Errorf(".git contents changed: %q", got)
	}
}

func TestMigrateHandlesMetadataNamesAtWorktreeRoot(t *testing.T) {
	for _, name := range []string{".revisa", ".git"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), name)
			id := localComment(t, root)
			initProject(t, root)
			out, stderr, code := run(t, root, "migrate")
			if code != 0 || stderr != "" ||
				out != "moved 1 comment from .revisa/\n" {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
			}
			got := listJSON(t, root)
			if len(got) != 1 || got[0]["id"] != id {
				t.Errorf("migrated comments: %v", got)
			}
		})
	}
}

func TestMigrateLeavesSymlinksAndDirectories(t *testing.T) {
	for _, name := range []string{
		"linked directory", "linked comment", "directory in store",
	} {
		t.Run(name, func(t *testing.T) {
			outside := t.TempDir()
			id := localComment(t, outside)
			external := filepath.Join(outside, ".revisa", id+".json")
			before := readFile(t, external)
			root := t.TempDir()
			initProject(t, root)
			local := filepath.Join(root, ".revisa")
			var link string
			switch name {
			case "linked directory":
				link = local
				if err := os.Symlink(filepath.Dir(external), link); err != nil {
					t.Fatal(err)
				}
			case "linked comment":
				if err := os.Mkdir(local, 0o755); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(local, id+".json")
				if err := os.Symlink(external, link); err != nil {
					t.Fatal(err)
				}
			case "directory in store":
				writeFile(t, local, "nested/.revisa/"+id+".json", before)
			}
			out, stderr, code := run(t, root, "migrate")
			if code != 3 || out != "no .revisa/ comments to move\n" ||
				stderr != "migrate: non-comment files found in .revisa/\n" {
				t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
			}
			if readFile(t, external) != before {
				t.Error("migration changed external comment")
			}
			if link != "" {
				info, err := os.Lstat(link)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Errorf("migration changed symlink: %v", err)
				}
			} else if readFile(t, filepath.Join(local,
				"nested", ".revisa", id+".json")) != before {
				t.Error("migration descended into a non-comment directory")
			}
			gitStore := filepath.Join(root, ".git", "revisa")
			if _, err := os.Stat(gitStore); !os.IsNotExist(err) {
				t.Errorf("non-comments created a git store: %v", err)
			}
		})
	}
}

func TestMigrateUsesCurrentWorktreeAndSkipsNestedGitfiles(t *testing.T) {
	p := normalClone(t)
	mainID := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	linkedID := "01JC8Z9A0B1C2D3E4F5G6H7J8K"
	for i, id := range []string{mainID, linkedID} {
		data := readFile(t, filepath.Join("..", "..", "testdata",
			"comments", id+".json"))
		writeFile(t, p.worktrees[i], ".revisa/"+id+".json", data)
	}
	linked := p.worktrees[1]
	writeFile(t, linked, "nested-project/.git", "gitdir: elsewhere\n")
	writeFile(t, linked, "nested-project/.revisa/ignored.json", "{")
	out, stderr, code := run(t, linked, "migrate")
	if code != 0 || stderr != "" ||
		out != "moved 1 comment from .revisa/\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, p.worktrees[0], "--branch", "feat/x")
	if len(got) != 1 || got[0]["id"] != linkedID ||
		got[0]["commit"] != "" || got[0]["path"] != "src/b.lua" {
		t.Errorf("shared git store: %v", got)
	}
	if _, err := os.Stat(filepath.Join(p.worktrees[0], ".revisa",
		mainID+".json")); err != nil {
		t.Errorf("migration touched the main worktree: %v", err)
	}
	local := filepath.Join(linked, ".revisa")
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Errorf("linked local store remains: %v", err)
	}
	if got := readFile(t, filepath.Join(linked, "nested-project",
		".revisa", "ignored.json")); got != "{" {
		t.Error("migration touched nested repository")
	}
}
