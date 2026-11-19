package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateIntoGitStore(t *testing.T) {
	root := t.TempDir()
	id := localComment(t, root)
	out, stderr, code := run(t, root, "resolve", id, "--reply", "Fixed.")
	if code != 0 || stderr != "" || out != "resolved "+id+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	file := filepath.Join(".revisa", id+".json")
	data := strings.Replace(readFile(t, filepath.Join(root, file)),
		`"path": "src/a.lua"`, `"path": "./src/a.lua"`, 1)
	writeFile(t, root, file, data)
	before := listJSON(t, root)[0]
	initProject(t, root)
	out, stderr, code = run(t, root, "migrate")
	if code != 0 || stderr != "" ||
		out != "moved 1 comment from .revisa/\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".revisa")); !os.IsNotExist(err) {
		t.Errorf("local store remains: %v", err)
	}
	got := listJSON(t, root)
	if len(got) != 1 || got[0]["id"] != id {
		t.Fatalf("migrated comments: %v", got)
	}
	before["branch"] = "main"
	for field, want := range before {
		if got[0][field] != want {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], want)
		}
	}
}

func TestMigrateDetachedHeadNeedsBranch(t *testing.T) {
	root := t.TempDir()
	id := localComment(t, root)
	path := filepath.Join(root, ".revisa", id+".json")
	before := strings.Replace(readFile(t, path),
		`"commit": ""`, `"commit": "obsolete"`, 1)
	writeFile(t, root, filepath.Join(".revisa", id+".json"), before)
	initProject(t, root)
	git(t, root, "checkout", "-q", "--detach")
	out, stderr, code := run(t, root, "migrate")
	if code != 1 || out != "" ||
		stderr != "revisa: migrate: HEAD is detached, pass --branch\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if readFile(t, path) != before {
		t.Error("detached migration changed source")
	}
	gitStore := filepath.Join(root, ".git", "revisa")
	if _, err := os.Stat(gitStore); !os.IsNotExist(err) {
		t.Errorf("failed migration created git store: %v", err)
	}
	out, stderr, code = run(t, root, "migrate", "--branch", "archived")
	if code != 0 || stderr != "" ||
		out != "moved 1 comment from .revisa/\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	got := listJSON(t, root, "--branch", "archived")
	if len(got) != 1 || got[0]["id"] != id ||
		got[0]["branch"] != "archived" || got[0]["commit"] != "" {
		t.Errorf("explicit branch migration: %v", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("local store remains: %v", err)
	}
}

func TestMigrateRequiresGitWorktree(t *testing.T) {
	t.Run("outside git", func(t *testing.T) {
		root := t.TempDir()
		id := localComment(t, root)
		path := filepath.Join(root, ".revisa", id+".json")
		before := readFile(t, path)
		for _, args := range [][]string{
			{"migrate"}, {"migrate", "--branch", "main"},
		} {
			out, stderr, code := run(t, root, args...)
			if code != 1 || out != "" ||
				stderr != "revisa: migrate: needs a git repository\n" {
				t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
			}
			if readFile(t, path) != before {
				t.Error("failed migration changed source")
			}
		}
	})
	t.Run("bare repository", func(t *testing.T) {
		root := t.TempDir()
		git(t, root, "init", "-q", "--bare", "-b", "main")
		out, stderr, code := run(t, root, "migrate")
		if code != 1 || out != "" ||
			stderr != "revisa: migrate: no working tree to walk\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
		}
	})
}

func TestMigrateNothingToMove(t *testing.T) {
	root := t.TempDir()
	initProject(t, root)
	for _, emptyDir := range []bool{false, true} {
		if emptyDir {
			path := filepath.Join(root, ".revisa")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		out, stderr, code := run(t, root, "migrate")
		if code != 0 || stderr != "" ||
			out != "no .revisa/ comments to move\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
		}
		for _, path := range []string{".revisa", ".git/revisa"} {
			full := filepath.Join(root, path)
			if _, err := os.Stat(full); !os.IsNotExist(err) {
				t.Errorf("empty migration left %s: %v", path, err)
			}
		}
	}
}

func TestMigrateLeavesClashesAndNonComments(t *testing.T) {
	root := t.TempDir()
	clash := localComment(t, root)
	movable := localComment(t, root)
	source := filepath.Join(root, ".revisa", clash+".json")
	before := readFile(t, source)
	writeFile(t, root, ".revisa/notes.txt", "not a comment")
	writeFile(t, root, ".revisa/broken.json", "{")
	writeFile(t, root, ".revisa/invalid.json", `{ "id": "../invalid" }`)
	initProject(t, root)
	existing := strings.Replace(before, "Keep this comment", "Existing", 1)
	destination := filepath.Join(root, ".git", "revisa", clash+".json")
	writeFile(t, root, filepath.Join(".git", "revisa", clash+".json"),
		existing)
	out, stderr, code := run(t, root, "migrate")
	wantWarnings := []string{
		"migrate: .revisa/" + clash + ".json: " +
			"comment id already exists in the git store\n",
		"migrate: non-comment files found in .revisa/\n",
	}
	if code != 3 || out != "moved 1 comment from .revisa/\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	for _, want := range wantWarnings {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q lacks %q", stderr, want)
		}
	}
	if readFile(t, destination) != existing || readFile(t, source) != before {
		t.Error("clash changed source or existing git comment")
	}
	for _, name := range []string{"notes.txt", "broken.json", "invalid.json"} {
		if _, err := os.Stat(filepath.Join(root, ".revisa", name)); err != nil {
			t.Errorf("non-comment %s did not stay: %v", name, err)
		}
	}
	movableSource := filepath.Join(root, ".revisa", movable+".json")
	if _, err := os.Stat(movableSource); !os.IsNotExist(err) {
		t.Errorf("movable source remains: %v", err)
	}
	got := listJSON(t, root)
	if len(got) != 1 || got[0]["id"] != movable {
		t.Errorf("movable comment missing: %v", got)
	}
}

func TestMigrateStoreFailurePreservesSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	for name, mode := range map[string]os.FileMode{
		"unreadable": 0o300, "unwritable": 0o500,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			id := localComment(t, root)
			source := filepath.Join(root, ".revisa", id+".json")
			before := readFile(t, source)
			initProject(t, root)
			addMocks(t, root)
			storeDir := filepath.Join(root, ".git", "revisa")
			if err := os.Chmod(storeDir, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(storeDir, 0o755) })
			out, stderr, code := run(t, root, "migrate")
			if code != 1 || out != "" ||
				!strings.Contains(stderr, "permission denied") {
				t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
			}
			if got := readFile(t, source); got != before {
				t.Error("failed store write changed source")
			}
			if err := os.Chmod(storeDir, 0o755); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(storeDir, id+".json")
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Errorf("failed migration published a comment: %v", err)
			}
		})
	}
}

func TestMigrateFailureReportsCompletedDirectories(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	localComment(t, root)
	initProject(t, root)
	clash := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	movable := "01JC8Z9A0B1C2D3E4F5G6H7J8K"
	local := "sub/.revisa/"
	writeFile(t, root, local+"a-clash.json", `{"id":"`+clash+`"}`)
	writeFile(t, root, local+"b-move.json", `{"id":"`+movable+`"}`)
	writeFile(t, root, local+"c-invalid.json", "{")
	writeFile(t, root, local+"d-unreadable.json", "{}")
	writeFile(t, root, ".git/revisa/"+clash+".json", "existing")
	unreadable := filepath.Join(root, local, "d-unreadable.json")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o644) })

	out, stderr, code := run(t, root, "migrate")
	want := "migrate: sub/.revisa/a-clash.json: comment id already " +
		"exists in the git store\nrevisa: open " + unreadable +
		": permission denied\n"
	if code != 1 || out != "moved 1 comment from .revisa/\n" ||
		stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	movedSource := filepath.Join(root, local, "b-move.json")
	if _, err := os.Stat(movedSource); !os.IsNotExist(err) {
		t.Errorf("successful move did not remove source: %v", err)
	}
	if got := readFile(t, filepath.Join(root, ".git", "revisa",
		clash+".json")); got != "existing" {
		t.Errorf("failed migration changed existing comment: %q", got)
	}
}

func TestMigrationUsage(t *testing.T) {
	out, stderr, code := run(t, t.TempDir(), "help")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, text := range []string{
		"list (ls)", "migrate", "3  migrate left comments in .revisa/",
	} {
		if !strings.Contains(out, text) {
			t.Errorf("usage %q lacks %q", out, text)
		}
	}
	fails(t, t.TempDir(), `migrate: unexpected argument "x"`, 2,
		"migrate", "x")
}
