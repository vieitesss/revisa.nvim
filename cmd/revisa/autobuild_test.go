package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goStub is a fake go on PATH. Its build writes a fake bin/revisa whose
// install only logs, so the tests never compile or touch $HOME.
const goStub = `#!/bin/sh
echo "$@" >> "$REVISA_GO_LOG"
if [ -n "$REVISA_GO_SLEEP" ]; then sleep "$REVISA_GO_SLEEP"; fi
if [ -n "$REVISA_GO_FAIL" ]; then echo "boom" >&2; exit 1; fi
mkdir -p bin
printf '#!/bin/sh\necho "$@" >> "$REVISA_INSTALL_LOG"\n' > bin/revisa
chmod +x bin/revisa
`

// delayedGitStub is a fake git on PATH. With REVISA_GIT_SLEEP set it
// answers the plugin's startup check only after a delay, so a test can run
// a command before the check returns; otherwise it runs the real git.
const delayedGitStub = `#!/bin/sh
if [ -z "$REVISA_GIT_SLEEP" ]; then exec "$REVISA_GIT" "$@"; fi
sleep "$REVISA_GIT_SLEEP"
printf '%s\n%s\n' "$REVISA_GIT_ROOT" "$REVISA_GIT_REV"
`

// buildPlugin is a copy of the plugin in its own git checkout, with a
// fake go first on PATH.
type buildPlugin struct {
	root, rev, goLog, installLog string
	env                          []string
}

func newBuildPlugin(t *testing.T) buildPlugin {
	t.Helper()
	root := copyPlugin(t)
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "plugin")
	stubs := t.TempDir()
	writeFile(t, stubs, "go", goStub)
	writeFile(t, stubs, "git", delayedGitStub)
	for _, name := range []string{"go", "git"} {
		if err := os.Chmod(filepath.Join(stubs, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	logs := t.TempDir()
	p := buildPlugin{
		root:       root,
		rev:        git(t, root, "rev-parse", "HEAD"),
		goLog:      filepath.Join(logs, "go"),
		installLog: filepath.Join(logs, "install"),
	}
	p.env = []string{
		"REVISA_PLUGIN=" + root,
		"REVISA_AUTO_BUILD=1",
		"REVISA_GO_LOG=" + p.goLog,
		"REVISA_INSTALL_LOG=" + p.installLog,
		"REVISA_GIT=" + gitBinary,
		"REVISA_GIT_ROOT=" + root,
		"REVISA_GIT_REV=" + p.rev,
		"PATH=" + strings.Join([]string{stubs, filepath.Dir(gitBinary),
			"/usr/bin", "/bin"}, string(os.PathListSeparator)),
	}
	return p
}

func logLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func stampOf(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "bin/.revisa-rev"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestAutoBuildBuildsAndInstallsWhenTheCommitIsNew(t *testing.T) {
	for _, kind := range []string{"no stamp", "older stamp"} {
		t.Run(kind, func(t *testing.T) {
			p := newBuildPlugin(t)
			if kind == "older stamp" {
				writeFile(t, p.root, "bin/.revisa-rev", "0123abc\n")
			}
			headless(t, t.TempDir(), "build.lua",
				append(p.env, "REVISA_CASE=built")...)
			want := []string{"build -o bin/revisa ./cmd/revisa"}
			if got := logLines(t, p.goLog); !equalLines(got, want) {
				t.Errorf("go ran %q, want %q", got, want)
			}
			got := logLines(t, p.installLog)
			if !equalLines(got, []string{"install"}) {
				t.Errorf("revisa ran %q, want install", got)
			}
			if got := stampOf(t, p.root); got != p.rev {
				t.Errorf("stamp %q, want %q", got, p.rev)
			}
		})
	}
}

func TestAutoBuildSkipsTheBuiltCommit(t *testing.T) {
	p := newBuildPlugin(t)
	writeFile(t, p.root, "bin/.revisa-rev", p.rev+"\n")
	writeFile(t, p.root, "bin/revisa", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(p.root, "bin/revisa"),
		0o755); err != nil {
		t.Fatal(err)
	}
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=idle")...)
	if got := logLines(t, p.goLog); got != nil {
		t.Errorf("go ran %q for the built commit", got)
	}
}

func TestAutoBuildIsOffWhenTheUserTurnsItOff(t *testing.T) {
	p := newBuildPlugin(t)
	env := append(p.env, "REVISA_CASE=idle")
	// The harness turns the build off unless REVISA_AUTO_BUILD is set.
	env = append(env, "REVISA_AUTO_BUILD=")
	headless(t, t.TempDir(), "build.lua", env...)
	if got := logLines(t, p.goLog); got != nil {
		t.Errorf("go ran %q with the build turned off", got)
	}
	if got := stampOf(t, p.root); got != "" {
		t.Errorf("stamp %q written with the build turned off", got)
	}
}

func TestAutoBuildReportsAFailedBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=failed", "REVISA_GO_FAIL=1")...)
	if got := logLines(t, p.installLog); got != nil {
		t.Errorf("revisa ran %q after a failed build", got)
	}
	if got := stampOf(t, p.root); got != "" {
		t.Errorf("stamp %q written after a failed build", got)
	}
}

func TestAutoBuildCommandRightAfterLoadReportsTheBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=immediate", "REVISA_GIT_SLEEP=1")...)
}

func TestAutoBuildCommandsWaitForTheBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=building", "REVISA_GO_SLEEP=1")...)
}

func equalLines(got, want []string) bool {
	return strings.Join(got, "\n") == strings.Join(want, "\n")
}
