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

var revisa string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "revisa-bin")
	if err != nil {
		panic(err)
	}
	revisa = filepath.Join(dir, "revisa")
	build := exec.Command("go", "build", "-o", revisa, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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
		data, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(storeDir, filepath.Base(m))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
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

// fails runs revisa and checks it exits non-zero with msg on stderr.
func fails(t *testing.T, dir, msg string, args ...string) {
	t.Helper()
	_, stderr, code := run(t, dir, args...)
	if code == 0 {
		t.Errorf("want non-zero exit")
	}
	if !strings.Contains(stderr, msg) {
		t.Errorf("stderr %q lacks %q", stderr, msg)
	}
}

const openOnFeatX = `01JC8Z7K3M9QX2V5T1R4N6P8WY src/a.lua:12-14 new abc123
Handle nil here

01JC8Z9A0B1C2D3E4F5G6H7J8K src/b.lua:3 old def456
Why was the cache removed?
Callers still expect it.
`

func TestListSeesCommentsFromEveryWorktree(t *testing.T) {
	setups := map[string]func(*testing.T) project{
		"normal clone": normalClone,
		"bare repo":    bareRepo,
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			p := setup(t)
			addMocks(t, p.worktrees[0])
			for _, wt := range p.worktrees {
				out, stderr, code := run(t, wt,
					"list", "--branch", "feat/x", "--status", "open")
				if code != 0 {
					t.Fatalf("exit %d: %s", code, stderr)
				}
				if out != openOnFeatX {
					t.Errorf("from %s got:\n%s\nwant:\n%s",
						wt, out, openOnFeatX)
				}
			}
		})
	}
}

func TestListFromSubdirectory(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	sub := filepath.Join(p.worktrees[1], "deep", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	out, _, code := run(t, sub,
		"list", "--branch", "feat/x", "--status", "open")
	if code != 0 || out != openOnFeatX {
		t.Errorf("exit %d, got:\n%s", code, out)
	}
}

func TestListResolvedShowsReply(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	out, _, _ := run(t, p.worktrees[1], "list", "--status", "resolved")
	want := `01JC8ZB0000000000000000000 README.md:1-2 new abc123 resolved
Typo in the title
reply: Fixed the title.
`
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestListDefaultsToCurrentBranch(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	out, _, _ := run(t, p.worktrees[1], "list")
	if n := strings.Count(out, "01JC8Z"); n != 3 {
		t.Errorf("want the 3 comments on feat/x, got %d:\n%s", n, out)
	}
	out, _, _ = run(t, p.worktrees[0], "list")
	if out != "" {
		t.Errorf("main has no comments, got:\n%s", out)
	}
	out, _, _ = run(t, p.worktrees[0], "list", "--branch", "feat/y")
	if n := strings.Count(out, "01JC8Z"); n != 1 {
		t.Errorf("want the 1 comment on feat/y, got %d:\n%s", n, out)
	}
}

func TestListOnDetachedHeadNeedsBranch(t *testing.T) {
	p := normalClone(t)
	git(t, p.worktrees[1], "checkout", "-q", "--detach")
	_, stderr, code := run(t, p.worktrees[1], "list")
	if code == 0 || !strings.Contains(stderr, "HEAD is detached") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestListJSONPrintsEverySchemaField(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	out, stderr, code := run(t, p.worktrees[0],
		"list", "--branch", "feat/x", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 comments on feat/x, got %d", len(got))
	}
	mock, err := os.ReadFile(
		"../../testdata/comments/01JC8Z7K3M9QX2V5T1R4N6P8WY.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(mock, &want); err != nil {
		t.Fatal(err)
	}
	for field, v := range want {
		gv, ok := got[0][field]
		if !ok {
			t.Errorf("missing field %q", field)
		} else if gv != v {
			t.Errorf("field %q: got %v, want %v", field, gv, v)
		}
	}
}

func TestListEmptyStore(t *testing.T) {
	p := normalClone(t)
	out, _, code := run(t, p.worktrees[0], "list")
	if code != 0 || out != "" {
		t.Errorf("exit %d, out %q", code, out)
	}
	out, _, code = run(t, p.worktrees[0], "list", "--json")
	if code != 0 || out != "[]\n" {
		t.Errorf("exit %d, out %q", code, out)
	}
}

func TestErrors(t *testing.T) {
	p := normalClone(t)
	cases := []struct {
		name string
		dir  string
		args []string
		msg  string
	}{
		{"no command", p.worktrees[0], nil, "usage: revisa"},
		{"unknown command", p.worktrees[0], []string{"frob"},
			`unknown command "frob"`},
		{"bad status", p.worktrees[0], []string{"list", "--status", "done"},
			"--status must be open or resolved"},
		{"unknown flag", p.worktrees[0], []string{"list", "--nope"},
			"flag provided but not defined: -nope"},
		{"stray argument", p.worktrees[0], []string{"list", "x"},
			`unexpected argument "x"`},
		{"outside git", t.TempDir(), []string{"list"},
			"not inside a git repository"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fails(t, c.dir, c.msg, c.args...)
		})
	}
}

func TestCorruptCommentNamesTheFile(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	bad := filepath.Join(p.worktrees[0], ".git", "revisa", "BROKEN.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, p.worktrees[0], "list")
	if code == 0 || !strings.Contains(stderr, "BROKEN.json") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestListReportsUnreadableStore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	p := normalClone(t)
	addMocks(t, p.worktrees[0])
	store := filepath.Join(p.worktrees[0], ".git", "revisa")
	if err := os.Chmod(store, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(store, 0o755) })
	fails(t, p.worktrees[1], "read store", "list", "--json")
}

func TestStoreIsUntrackedAndSurvivesClean(t *testing.T) {
	p := normalClone(t)
	addMocks(t, p.worktrees[1])
	for _, wt := range p.worktrees {
		if s := git(t, wt, "status", "--porcelain", "--ignored"); s != "" {
			t.Errorf("git status in %s shows %q", wt, s)
		}
		git(t, wt, "clean", "-fdx")
	}
	out, _, _ := run(t, p.worktrees[1], "list")
	if n := strings.Count(out, "01JC8Z"); n != 3 {
		t.Errorf("want 3 comments after git clean, got %d", n)
	}
}
