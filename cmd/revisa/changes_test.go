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

type worktreeJSON struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Head    string `json:"head"`
	Current bool   `json:"current"`
}

type changeJSON struct {
	Status  string `json:"status"`
	Path    string `json:"path"`
	OldPath string `json:"old_path"`
}

type changesJSON struct {
	Worktree      string       `json:"worktree"`
	Branch        string       `json:"branch"`
	Head          string       `json:"head"`
	DefaultBranch string       `json:"default_branch"`
	Base          string       `json:"base"`
	Entries       []changeJSON `json:"entries"`
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// worktrees runs revisa worktrees --json in dir.
func worktrees(t *testing.T, dir string) []worktreeJSON {
	t.Helper()
	out, stderr, code := run(t, dir, "worktrees", "--json")
	if code != 0 {
		t.Fatalf("worktrees exit %d: %s", code, stderr)
	}
	var got []worktreeJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return got
}

// changes runs revisa changes --json in dir.
func changes(t *testing.T, dir string, args ...string) changesJSON {
	t.Helper()
	out, stderr, code := run(t, dir,
		append([]string{"changes", "--json"}, args...)...)
	if code != 0 {
		t.Fatalf("changes exit %d: %s", code, stderr)
	}
	var got changesJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return got
}

// changesProject is a project whose feat/x worktree holds every kind of
// session change: a commit, an uncommitted edit, a deletion, a rename
// and an untracked file.
func changesProject(t *testing.T) (p project, linked string) {
	t.Helper()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	git(t, root, "init", "-q", "-b", "main", main)
	writeFile(t, main, "src/a.lua", committedLua)
	writeFile(t, main, "src/gone.go", "package gone\n")
	writeFile(t, main, "src/moved.go", strings.Repeat("line\n", 20))
	git(t, main, "add", ".")
	git(t, main, "commit", "-q", "-m", "init")
	linked = filepath.Join(root, "linked")
	git(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	writeFile(t, linked, "src/new.go", "package new\n")
	git(t, linked, "add", "src/new.go")
	git(t, linked, "commit", "-q", "-m", "add new.go")
	writeFile(t, linked, "src/a.lua", editedLua)
	if err := os.Remove(filepath.Join(linked, "src/gone.go")); err != nil {
		t.Fatal(err)
	}
	git(t, linked, "mv", "src/moved.go", "src/renamed.go")
	writeFile(t, linked, "src/renamed.go",
		strings.Repeat("line\n", 21))
	writeFile(t, linked, "notes.txt", "scratch\n")
	return project{worktrees: []string{main, linked}}, linked
}

func TestWorktreesListsEveryWorktree(t *testing.T) {
	p := normalClone(t)
	deep := filepath.Join(p.worktrees[1], "deep", "dir")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got := worktrees(t, deep)
	want := map[string]worktreeJSON{
		realPath(t, p.worktrees[0]): {
			Branch: "main",
			Head:   git(t, p.worktrees[0], "rev-parse", "HEAD"),
		},
		realPath(t, p.worktrees[1]): {
			Branch:  "feat/x",
			Head:    git(t, p.worktrees[1], "rev-parse", "HEAD"),
			Current: true,
		},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d worktrees, got %d: %v", len(want), len(got), got)
	}
	for _, wt := range got {
		expected, ok := want[wt.Path]
		if !ok {
			t.Errorf("unexpected worktree %q", wt.Path)
			continue
		}
		expected.Path = wt.Path
		if wt != expected {
			t.Errorf("got %+v, want %+v", wt, expected)
		}
	}
}

func TestWorktreesSkipsBareAndMarksDetached(t *testing.T) {
	p := bareRepo(t)
	git(t, p.worktrees[1], "checkout", "-q", "--detach")
	got := worktrees(t, p.worktrees[0])
	if len(got) != 2 {
		t.Fatalf("want the 2 worktrees, got %v", got)
	}
	for _, wt := range got {
		switch wt.Path {
		case realPath(t, p.worktrees[0]):
			if wt.Branch != "main" || !wt.Current {
				t.Errorf("wrong main entry %+v", wt)
			}
		case realPath(t, p.worktrees[1]):
			if wt.Branch != "" || wt.Current {
				t.Errorf("wrong detached entry %+v", wt)
			}
		default:
			t.Errorf("bare entry listed: %q", wt.Path)
		}
	}
}

func TestWorktreesPlainText(t *testing.T) {
	p := normalClone(t)
	out, stderr, code := run(t, p.worktrees[1], "worktrees")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := "main " + realPath(t, p.worktrees[0]) + "\n" +
		"feat/x " + realPath(t, p.worktrees[1]) + " (current)\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestWorktreesOutsideGitFails(t *testing.T) {
	fails(t, t.TempDir(), "needs a git repository", 1, "worktrees")
}

func TestChangesListsEveryKindOfSessionChange(t *testing.T) {
	p, linked := changesProject(t)
	got := changes(t, p.worktrees[0], "--worktree", linked)
	if want := realPath(t, linked); got.Worktree != want {
		t.Errorf("worktree %q, want %q", got.Worktree, want)
	}
	if got.Branch != "feat/x" {
		t.Errorf("branch %q, want feat/x", got.Branch)
	}
	if got.DefaultBranch != "main" {
		t.Errorf("default_branch %q, want main", got.DefaultBranch)
	}
	if want := git(t, linked, "merge-base", "main", "HEAD"); got.Base != want {
		t.Errorf("base %q, want %q", got.Base, want)
	}
	want := []changeJSON{
		{Status: "?", Path: "notes.txt"},
		{Status: "M", Path: "src/a.lua"},
		{Status: "D", Path: "src/gone.go"},
		{Status: "A", Path: "src/new.go"},
		{Status: "R", Path: "src/renamed.go", OldPath: "src/moved.go"},
	}
	if len(got.Entries) != len(want) {
		t.Fatalf("want %v, got %v", want, got.Entries)
	}
	for i := range want {
		if got.Entries[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v",
				i, got.Entries[i], want[i])
		}
	}
}

func TestChangesReportsTheHeadOfTheWorktree(t *testing.T) {
	p, linked := changesProject(t)
	got := changes(t, p.worktrees[0], "--worktree", linked)
	if want := git(t, linked, "rev-parse", "HEAD"); got.Head != want {
		t.Errorf("head %q, want %q", got.Head, want)
	}
}

func TestChangesOnDefaultBranchComparesWithHead(t *testing.T) {
	p := normalClone(t)
	main := p.worktrees[0]
	writeFile(t, main, "src/a.lua", "one\n")
	git(t, main, "add", ".")
	git(t, main, "commit", "-q", "-m", "add a.lua")
	writeFile(t, main, "src/a.lua", "two\n")
	writeFile(t, main, "notes.txt", "scratch\n")
	got := changes(t, main)
	head := git(t, main, "rev-parse", "HEAD")
	if got.Base != head {
		t.Errorf("base %q, want HEAD %q", got.Base, head)
	}
	want := []changeJSON{
		{Status: "?", Path: "notes.txt"},
		{Status: "M", Path: "src/a.lua"},
	}
	if len(got.Entries) != len(want) {
		t.Fatalf("want %v, got %v", want, got.Entries)
	}
	for i := range want {
		if got.Entries[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v",
				i, got.Entries[i], want[i])
		}
	}
}

func TestChangesFindsDefaultBranchFromOriginHead(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	git(t, root, "init", "-q", "-b", "trunk", seed)
	writeFile(t, seed, "base.txt", "base\n")
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-q", "-m", "init")
	clone := filepath.Join(root, "main")
	git(t, root, "clone", "-q", seed, clone)
	git(t, clone, "remote", "set-head", "origin", "trunk")
	linked := filepath.Join(root, "linked")
	git(t, clone, "worktree", "add", "-q", "-b", "feat/x", linked)
	writeFile(t, linked, "new.txt", "new\n")
	git(t, linked, "add", ".")
	git(t, linked, "commit", "-q", "-m", "new")

	got := changes(t, clone, "--worktree", "feat/x")
	if got.DefaultBranch != "trunk" {
		t.Errorf("default_branch %q, want trunk", got.DefaultBranch)
	}
	want := git(t, linked, "merge-base", "trunk", "HEAD")
	if got.Base != want {
		t.Errorf("base %q, want %q", got.Base, want)
	}
	if len(got.Entries) != 1 ||
		got.Entries[0] != (changeJSON{Status: "A", Path: "new.txt"}) {
		t.Errorf("entries %v", got.Entries)
	}
}

func TestChangesFindsDefaultBranchFromInitDefault(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	git(t, root, "init", "-q", "-b", "trunk", main)
	git(t, main, "config", "init.defaultBranch", "trunk")
	writeFile(t, main, "base.txt", "base\n")
	git(t, main, "add", ".")
	git(t, main, "commit", "-q", "-m", "init")
	linked := filepath.Join(root, "linked")
	git(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	writeFile(t, linked, "base.txt", "changed\n")

	got := changes(t, linked)
	if got.DefaultBranch != "trunk" {
		t.Errorf("default_branch %q, want trunk", got.DefaultBranch)
	}
	if want := git(t, main, "rev-parse", "HEAD"); got.Base != want {
		t.Errorf("base %q, want %q", got.Base, want)
	}
}

func TestChangesErrorsWithoutDefaultBranch(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "trunk")
	git(t, root, "commit", "-q", "--allow-empty", "-m", "init")
	_, stderr, code := run(t, root, "changes")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, name := range []string{"origin/HEAD", "init.defaultBranch",
		"main", "master"} {
		if !strings.Contains(stderr, name) {
			t.Errorf("stderr %q does not name %q", stderr, name)
		}
	}
}

func TestChangesPlainText(t *testing.T) {
	p, linked := changesProject(t)
	base := git(t, linked, "merge-base", "main", "HEAD")
	out, stderr, code := run(t, p.worktrees[0],
		"changes", "--worktree", linked)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := "worktree " + realPath(t, linked) + " feat/x base " +
		base[:7] + " (main)\n" +
		"? notes.txt\n" +
		"M src/a.lua\n" +
		"D src/gone.go\n" +
		"A src/new.go\n" +
		"R src/moved.go -> src/renamed.go\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestChangesOfWorktreeOutsideCwd(t *testing.T) {
	p, linked := changesProject(t)
	fromCwd := changes(t, p.worktrees[1], "--worktree",
		p.worktrees[0])
	if fromCwd.Worktree != realPath(t, p.worktrees[0]) {
		t.Errorf("worktree %q, want %q",
			fromCwd.Worktree, realPath(t, p.worktrees[0]))
	}
	if len(fromCwd.Entries) != 0 {
		t.Errorf("main has no session changes, got %v", fromCwd.Entries)
	}
	byBranch := changes(t, p.worktrees[0], "--worktree", "feat/x")
	if byBranch.Worktree != realPath(t, linked) ||
		byBranch.Branch != "feat/x" {
		t.Errorf("by branch got %+v", byBranch)
	}
	if len(byBranch.Entries) != 5 {
		t.Errorf("by branch entries %v", byBranch.Entries)
	}
}

func TestChangesUnknownWorktreeFails(t *testing.T) {
	p := normalClone(t)
	fails(t, p.worktrees[0], `"nope"`, 1, "changes", "--worktree", "nope")
	missing := filepath.Join(p.worktrees[0], "nope")
	fails(t, p.worktrees[0], `"`+missing+`"`, 1,
		"changes", "--worktree", missing)
}

func TestChangesOutsideGitFails(t *testing.T) {
	fails(t, t.TempDir(), "needs a git repository", 1, "changes")
}

func TestChangesDetachedWorktree(t *testing.T) {
	p, linked := changesProject(t)
	git(t, linked, "checkout", "-q", "--detach")
	got := changes(t, p.worktrees[0], "--worktree", linked)
	if got.Branch != "" {
		t.Errorf("branch %q, want empty", got.Branch)
	}
	out, stderr, code := run(t, p.worktrees[0],
		"changes", "--worktree", linked)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.HasPrefix(out, "worktree "+realPath(t, linked)+
		" detached base ") {
		t.Errorf("header %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestChangesUsageErrors(t *testing.T) {
	p := normalClone(t)
	fails(t, p.worktrees[0], "unexpected argument", 2,
		"changes", "extra")
	fails(t, p.worktrees[0], "unexpected argument", 2,
		"worktrees", "extra")
	fails(t, p.worktrees[0], "flag needs an argument", 2,
		"changes", "--worktree")
}

func TestReviewStartsNeovim(t *testing.T) {
	cases := map[string]struct {
		pick       bool
		nvimEnvVar bool
	}{
		"picker":          {pick: true},
		"worktree":        {},
		"REVISA_NVIM set": {nvimEnvVar: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := normalClone(t)
			linked := p.worktrees[1]
			stub := t.TempDir()
			record := filepath.Join(stub, "cwd")
			worktreeRecord := filepath.Join(stub, "worktree")
			writeFile(t, stub, "nvim",
				"#!/bin/sh\npwd > \"$REVISA_RECORD\"\n"+
					"printf '%s' \"${REVISA_REVIEW_WORKTREE-}\" > "+
					"\"$REVISA_WORKTREE_RECORD\"\n")
			if err := os.Chmod(filepath.Join(stub, "nvim"),
				0o755); err != nil {
				t.Fatal(err)
			}
			env := []string{"PATH=" + stub, "REVISA_RECORD=" + record,
				"REVISA_WORKTREE_RECORD=" + worktreeRecord}
			if c.nvimEnvVar {
				env = []string{"PATH=/nonexistent",
					"REVISA_NVIM=" + filepath.Join(stub, "nvim"),
					"REVISA_RECORD=" + record,
					"REVISA_WORKTREE_RECORD=" + worktreeRecord}
			}
			args := []string{"review"}
			// The worktree reaches the editor verbatim through the
			// documented environment variable, not an Ex command.
			wantWorktree := ""
			if !c.pick {
				args = append(args, linked)
				wantWorktree = linked
			}
			_, stderr, code := runEnv(t, linked, env, args...)
			if code != 0 {
				t.Fatalf("review exit %d: %s", code, stderr)
			}
			got := strings.TrimSuffix(readFile(t, record), "\n")
			if want := realPath(t, linked); got != want {
				t.Errorf("editor cwd %q, want %q", got, want)
			}
			if got := readFile(t, worktreeRecord); got != wantWorktree {
				t.Errorf("REVISA_REVIEW_WORKTREE %q, want %q",
					got, wantWorktree)
			}
		})
	}
}

// TestReviewOpensTheWorktreeInNeovim covers the characters an Ex command
// line would mangle: the worktree reaches Neovim verbatim, so the plugin's
// CLI lists that worktree and the changes buffer names its branch.
func TestReviewOpensTheWorktreeInNeovim(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Fatal("review tests require Neovim 0.12+ on PATH")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := `#!/bin/sh
exec "$REVISA_REAL_NVIM" --headless --clean \
    --cmd "lua vim.opt.rtp:prepend(vim.env.REVISA_PLUGIN)" \
    --cmd "set loadplugins" \
    "$@" -c "lua dofile(vim.env.REVISA_INSPECT)"
`
	for _, name := range []string{
		"with space", "with|pipe", "with'quote", `with"quote"`,
	} {
		t.Run(name, func(t *testing.T) {
			p, linked := changesProject(t)
			dir := filepath.Join(t.TempDir(), name)
			git(t, linked, "worktree", "add", "-q", "-b", "feat/w", dir)
			stub := t.TempDir()
			writeFile(t, stub, "nvim", wrapper)
			if err := os.Chmod(filepath.Join(stub, "nvim"),
				0o755); err != nil {
				t.Fatal(err)
			}
			path := strings.Join([]string{stub,
				filepath.Dir(revisa), filepath.Dir(gitBinary)},
				string(os.PathListSeparator))
			_, stderr, code := runEnv(t, p.worktrees[0], []string{
				"PATH=" + path,
				"REVISA_NVIM=" + filepath.Join(stub, "nvim"),
				"REVISA_REAL_NVIM=" + nvim,
				"REVISA_PLUGIN=" + root,
				"REVISA_INSPECT=" + filepath.Join(root,
					"testdata/nvim/review-worktree.lua"),
				"REVISA_NAME=revisa://changes/feat/w",
				"REVISA_EXPECT=" + realPath(t, dir),
			}, "review", dir)
			if code != 0 {
				t.Fatalf("review exit %d: %s", code, stderr)
			}
		})
	}
}

// runEnv runs revisa with env added to a copy of the environment that
// has no PATH, so a test controls the shell path completely.
func runEnv(
	t *testing.T, dir string, env []string, args ...string,
) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(revisa, args...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PATH=") ||
			strings.HasPrefix(entry, "REVISA_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, env...)
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
