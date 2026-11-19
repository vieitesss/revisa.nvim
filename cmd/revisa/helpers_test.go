package main_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

type project struct {
	worktrees []string
}

func normalClone(t *testing.T) project {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	git(t, root, "init", "-q", "-b", "main", main)
	git(t, main, "commit", "-q", "--allow-empty", "-m", "init")
	linked := filepath.Join(root, "linked")
	git(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	return project{worktrees: []string{main, linked}}
}

func bareRepo(t *testing.T) project {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	git(t, root, "init", "-q", "-b", "main", seed)
	git(t, seed, "commit", "-q", "--allow-empty", "-m", "init")
	bare := filepath.Join(root, "repo.git")
	git(t, root, "clone", "-q", "--bare", seed, bare)
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	git(t, bare, "worktree", "add", "-q", a, "main")
	git(t, bare, "worktree", "add", "-q", "-b", "feat/x", b)
	return project{worktrees: []string{a, b}}
}

func addMocks(t *testing.T, worktree string) {
	t.Helper()
	common := git(t, worktree,
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	storeDir := filepath.Join(common, "revisa")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mocks, err := filepath.Glob("../../testdata/comments/*.json")
	if err != nil || len(mocks) == 0 {
		t.Fatalf("no mocks: %v", err)
	}
	for _, m := range mocks {
		writeFile(t, storeDir, filepath.Base(m), readFile(t, m))
	}
}

func run(
	t *testing.T, dir string, args ...string,
) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(revisa, args...)
	cmd.Dir = dir
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code
}

func fails(t *testing.T, dir, msg string, want int, args ...string) {
	t.Helper()
	_, stderr, code := run(t, dir, args...)
	if code != want {
		t.Errorf("exit %d, want %d", code, want)
	}
	if !strings.Contains(stderr, msg) {
		t.Errorf("stderr %q lacks %q", stderr, msg)
	}
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func listJSON(t *testing.T, dir string, args ...string) []map[string]any {
	t.Helper()
	out, stderr, code := run(t, dir, append([]string{"list", "--json"},
		args...)...)
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return got
}

func addOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, stderr, code := run(t, dir, append([]string{"add"}, args...)...)
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}
	return strings.TrimSpace(out)
}

func localComment(t *testing.T, root string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".revisa"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "src/a.lua", "anchor\n")
	return addOK(t, root, "--path", "src/a.lua", "--lines", "1",
		"--body", "Keep this comment")
}

func initProject(t *testing.T, root string) {
	t.Helper()
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "commit", "-q", "--allow-empty", "-m", "init")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
