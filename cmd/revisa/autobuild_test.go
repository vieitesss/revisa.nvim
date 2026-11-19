package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const goStub = `#!/bin/sh
echo "$@" >> "$REVISA_GO_LOG"
if [ -n "$REVISA_GO_SLEEP" ]; then sleep "$REVISA_GO_SLEEP"; fi
if [ -n "$REVISA_GO_FAIL" ]; then echo "boom" >&2; exit 1; fi
mkdir -p bin
printf '#!/bin/sh\necho "$@" >> "$REVISA_INSTALL_LOG"\n' > bin/revisa
chmod +x bin/revisa
`

const delayedGitStub = `#!/bin/sh
if [ -z "$REVISA_GIT_SLEEP" ]; then exec "$REVISA_GIT" "$@"; fi
sleep "$REVISA_GIT_SLEEP"
printf '%s\n%s\n' "$REVISA_GIT_ROOT" "$REVISA_GIT_REV"
`

type buildPlugin struct {
	root, goLog, installLog string
	env                     []string
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
		"REVISA_GIT_REV=" + git(t, root, "rev-parse", "HEAD"),
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

func TestAutoBuildBuildsAndInstallsWhenTheCommitIsNew(t *testing.T) {
	p := newBuildPlugin(t)
	wantBuilds, wantInstalls := []string{}, []string{}
	for _, step := range []string{"install", "restart", "update"} {
		t.Run(step, func(t *testing.T) {
			if step == "update" {
				git(t, p.root, "commit", "-q", "--allow-empty",
					"-m", "update")
			}
			outcome := "built"
			if step == "restart" {
				outcome = "idle"
			} else {
				wantBuilds = append(wantBuilds,
					"build -o bin/revisa ./cmd/revisa")
				wantInstalls = append(wantInstalls, "install")
			}
			headless(t, t.TempDir(), "build.lua",
				append(p.env, "REVISA_CASE="+outcome)...)
			if got := logLines(t, p.goLog); !slices.Equal(got, wantBuilds) {
				t.Errorf("go ran %q, want %q", got, wantBuilds)
			}
			got := logLines(t, p.installLog)
			if !slices.Equal(got, wantInstalls) {
				t.Errorf("revisa ran %q, want %q", got, wantInstalls)
			}
		})
	}
}

func TestAutoBuildIsOffWhenTheUserTurnsItOff(t *testing.T) {
	p := newBuildPlugin(t)
	env := append(p.env, "REVISA_CASE=idle", "REVISA_AUTO_BUILD=")
	headless(t, t.TempDir(), "build.lua", env...)
	if got := logLines(t, p.goLog); got != nil {
		t.Errorf("go ran %q with the build turned off", got)
	}
	if got := logLines(t, p.installLog); got != nil {
		t.Errorf("revisa ran %q with the build turned off", got)
	}
}

func TestAutoBuildReportsAFailedBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=failed", "REVISA_GO_FAIL=1")...)
	if got := logLines(t, p.installLog); got != nil {
		t.Errorf("revisa ran %q after a failed build", got)
	}
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=built")...)
	want := []string{
		"build -o bin/revisa ./cmd/revisa",
		"build -o bin/revisa ./cmd/revisa",
	}
	if got := logLines(t, p.goLog); !slices.Equal(got, want) {
		t.Errorf("go ran %q, want a retry: %q", got, want)
	}
	if got := logLines(t, p.installLog); !slices.Equal(got,
		[]string{"install"}) {
		t.Errorf("revisa ran %q, want one successful install", got)
	}
}

func TestAutoBuildCommandRightAfterLoadReportsTheBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=immediate", "REVISA_GIT_SLEEP=1")...)
}

func TestAutoBuildCommandsReportTheRunningBuild(t *testing.T) {
	p := newBuildPlugin(t)
	headless(t, t.TempDir(), "build.lua",
		append(p.env, "REVISA_CASE=building", "REVISA_GO_SLEEP=1")...)
}
