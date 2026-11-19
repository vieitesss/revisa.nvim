package project_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/project"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

// gitAt runs git in dir with a fixed identity and no user config.
func gitAt(t *testing.T, dir string, args ...string) string {
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

// clone makes a repository with a main worktree and a second directory
// holding the feat/x branch.
func clone(t *testing.T) (main, linked string) {
	t.Helper()
	root := t.TempDir()
	main = filepath.Join(root, "main")
	gitAt(t, root, "init", "-q", "-b", "main", main)
	gitAt(t, main, "commit", "-q", "--allow-empty", "-m", "init")
	linked = filepath.Join(root, "linked")
	gitAt(t, main, "worktree", "add", "-q", "-b", "feat/x", linked)
	return main, linked
}

// bareRepo makes a bare repository with worktrees on main and feat/x.
func bareRepo(t *testing.T) (main, linked string) {
	t.Helper()
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	gitAt(t, root, "init", "-q", "-b", "main", seed)
	gitAt(t, seed, "commit", "-q", "--allow-empty", "-m", "init")
	bare := filepath.Join(root, "repo.git")
	gitAt(t, root, "clone", "-q", "--bare", seed, bare)
	main = filepath.Join(root, "a")
	gitAt(t, bare, "worktree", "add", "-q", main, "main")
	linked = filepath.Join(root, "b")
	gitAt(t, bare, "worktree", "add", "-q", "-b", "feat/x", linked)
	return main, linked
}

// writeFile writes path under dir, creating its directories.
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

// commitFile writes path under dir and commits it.
func commitFile(t *testing.T, dir, path, content string) string {
	t.Helper()
	writeFile(t, dir, path, content)
	gitAt(t, dir, "add", ".")
	gitAt(t, dir, "commit", "-q", "-m", "write "+path)
	return gitAt(t, dir, "rev-parse", "HEAD")
}

// find returns the project containing dir.
func find(t *testing.T, dir string) project.Project {
	t.Helper()
	p, err := project.Find(dir)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	return p
}

// add writes an open comment over path at line start.
func add(t *testing.T, p project.Project, path string, start int, body string,
) store.Comment {
	t.Helper()
	c, err := p.Add(project.Draft{
		Path:  path,
		Start: start,
		End:   start,
		Body:  body,
		Side:  "new",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	return c
}

// list returns p's comments on branch, filtered by status.
func list(t *testing.T, p project.Project, branch, status string,
) []project.LocatedComment {
	t.Helper()
	got, err := p.List(branch, status, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return got
}

// only returns the single comment in got.
func only(t *testing.T, got []project.LocatedComment) project.LocatedComment {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %+v", got)
	}
	return got[0]
}

// storeFiles maps each store file name to its content.
func storeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		files[e.Name()] = readFile(t, filepath.Join(dir, e.Name()))
	}
	return files
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
