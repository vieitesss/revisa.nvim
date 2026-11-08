package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitStub returns a directory holding a git wrapper that logs the
// arguments of every invocation to log, then runs the real git. Putting
// the directory first on PATH lets a test count the git processes revisa
// runs.
func gitStub(t *testing.T) (dir, log string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	stub := t.TempDir()
	log = filepath.Join(stub, "log")
	writeFile(t, stub, "git", "#!/bin/sh\n"+
		`printf '%s\n' "$*" >> "`+log+`"`+"\n"+
		`exec "`+real+`" "$@"`+"\n")
	if err := os.Chmod(filepath.Join(stub, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub, log
}

func TestListReadsWorktreesOnce(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	for line := range 4 {
		addOK(t, linked, "--path", "src/a.lua", "--lines",
			strconv.Itoa(line+1), "--body", "Comment "+strconv.Itoa(line))
	}
	stub, log := gitStub(t)
	out, stderr, code := runEnv(t, linked, []string{"PATH=" + stub},
		"list", "--json")
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	if n := strings.Count(out, `"id"`); n != 4 {
		t.Fatalf("want 4 comments, got %d:\n%s", n, out)
	}
	lines := strings.Split(strings.TrimSuffix(readFile(t, log), "\n"), "\n")
	worktreeLists := 0
	for _, line := range lines {
		if strings.Contains(line, "worktree list") {
			worktreeLists++
		}
	}
	if worktreeLists != 1 {
		t.Errorf("read the worktree list %d times, want once:\n%s",
			worktreeLists, strings.Join(lines, "\n"))
	}
}
