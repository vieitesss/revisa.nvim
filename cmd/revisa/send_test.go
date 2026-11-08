package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// recadoStubScript stands in for recado. "agents" prints agents.json;
// "send" records its argv and address in send<N> and addr<N>, then exits
// with the code in code<N>, 0 by default. Only shell builtins are used,
// so a test PATH can leave out the real recado.
const recadoStubScript = `#!/bin/sh
dir=${RECADO_STUB:?}
if [ "$1" = agents ]; then
    while IFS= read -r line || [ -n "$line" ]; do
        printf '%s\n' "$line"
    done <"$dir/agents.json"
    exit 0
fi
n=0
if [ -f "$dir/count" ]; then
    read -r n <"$dir/count"
fi
n=$((n + 1))
printf '%s\n' "$n" >"$dir/count"
: >"$dir/send$n"
for arg in "$@"; do
    printf '%s\n' "$arg" >>"$dir/send$n"
done
printf '%s\n' "${RECADO_ADDR-}" >"$dir/addr$n"
code=0
if [ -f "$dir/code$n" ]; then
    read -r code <"$dir/code$n"
fi
if [ -f "$dir/stderr$n" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
        printf '%s\n' "$line" >&2
    done <"$dir/stderr$n"
fi
exit "$code"
`

type stubAgent struct {
	ID      string `json:"id"`
	Harness string `json:"harness"`
	Live    bool   `json:"live"`
	Cwd     string `json:"cwd,omitempty"`
	Machine string `json:"machine,omitempty"`
	Remote  string `json:"remote,omitempty"`
}

// agentsJSON is what the stub recado prints for "agents --json".
func agentsJSON(t *testing.T, agents ...stubAgent) string {
	t.Helper()
	if agents == nil {
		agents = []stubAgent{}
	}
	data, err := json.Marshal(map[string]any{"agents": agents})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// recadoStub writes the stub into a directory of its own, so it shadows
// any recado installed on the machine.
func recadoStub(t *testing.T, agents string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "recado", recadoStubScript)
	if err := os.Chmod(filepath.Join(dir, "recado"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "agents.json", agents)
	return dir
}

// stubCodes sets the exit code of the given send calls, in order.
func stubCodes(t *testing.T, stub string, codes ...int) {
	t.Helper()
	for i, code := range codes {
		writeFile(t, stub, "code"+strconv.Itoa(i+1),
			strconv.Itoa(code)+"\n")
	}
}

func stubStderr(t *testing.T, stub string, call int, message string) {
	t.Helper()
	writeFile(t, stub, "stderr"+strconv.Itoa(call), message+"\n")
}

// sendPath is the test PATH: the stub first, then the plugin's binary and
// git. With no stub, nothing named recado is reachable.
func sendPath(t *testing.T, stub string) string {
	t.Helper()
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{}
	if stub != "" {
		parts = append(parts, stub)
	}
	parts = append(parts, filepath.Dir(revisa), filepath.Dir(gitBinary))
	return strings.Join(parts, string(os.PathListSeparator))
}

// stubCalls is how many send calls the stub received.
func stubCalls(t *testing.T, stub string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stub, "count"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	calls, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("stub count: %v", err)
	}
	return calls
}

// stubArgv is the argv of one recorded send call.
func stubArgv(t *testing.T, stub string, call int) []string {
	t.Helper()
	data, err := os.ReadFile(
		filepath.Join(stub, "send"+strconv.Itoa(call)))
	if err != nil {
		t.Fatalf("no recorded send call %d: %v", call, err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// stubAddr is RECADO_ADDR as one recorded send call saw it.
func stubAddr(t *testing.T, stub string, call int) string {
	t.Helper()
	data, err := os.ReadFile(
		filepath.Join(stub, "addr"+strconv.Itoa(call)))
	if err != nil {
		t.Fatalf("no recorded send call %d: %v", call, err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func assertNoSend(t *testing.T, stub string) {
	t.Helper()
	if calls := stubCalls(t, stub); calls != 0 {
		t.Fatalf("recado was called %d times", calls)
	}
}

// wantMessage is the notification an agent reads, with the branch quoted
// for a shell.
func wantMessage(branch string) string {
	quoted := "'" + strings.ReplaceAll(branch, "'", `'\''`) + "'"
	return "revisa: review comments are waiting on branch " + quoted +
		". Run: revisa list --branch " + quoted +
		" --status open. Answer each with revisa resolve, not recado reply."
}

// wantArgv is the argv of one send call.
func wantArgv(id, branch string) []string {
	return []string{
		"send", id, wantMessage(branch), "--meta",
		"revisa_branch=" + branch,
	}
}

func equalStrings(t *testing.T, what string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: got %q, want %q", what, got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s: got %q, want %q", what, got, want)
		}
	}
}

// send runs send.lua headless in dir with the stub on PATH.
func sendCase(t *testing.T, dir, stub string, env ...string) {
	t.Helper()
	headless(t, dir, "send.lua", append([]string{
		"RECADO_STUB=" + stub,
		"PATH=" + sendPath(t, stub),
	}, env...)...)
}

func TestBufferSendUnavailable(t *testing.T) {
	cases := map[string]struct {
		setting string
		stub    bool
		want    string
	}{
		"notify none": {
			setting: "none",
			stub:    true,
			want:    `setup({ notify = "recado" | "rex" })`,
		},
		"recado missing": {
			setting: "recado",
			want:    "recado is not on PATH",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			stub := recadoStub(t, agentsJSON(t))
			path := ""
			if c.stub {
				path = stub
			}
			sendCase(t, linked, stub,
				"REVISA_CASE=unavailable",
				"REVISA_SETTING="+c.setting,
				"REVISA_ERROR="+c.want,
				"PATH="+sendPath(t, path))
			assertNoSend(t, stub)
		})
	}
}

func TestBufferSendValidatesTheSetting(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	stub := recadoStub(t, agentsJSON(t))
	sendCase(t, linked, stub,
		"REVISA_CASE=invalid",
		"REVISA_SETTING=bogus",
		"REVISA_ERROR="+`setup({ notify = "recado" | "rex" })`)
	assertNoSend(t, stub)
}

func TestBufferSendPrefersTheWorktreeMatch(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(linked, link); err != nil {
		t.Fatal(err)
	}
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "gone", Harness: "claude", Cwd: linked},
		stubAgent{ID: "codex", Harness: "codex", Live: true, Cwd: linked},
		stubAgent{ID: "elsewhere", Harness: "claude", Live: true,
			Cwd: linked, Machine: "other"},
		stubAgent{ID: "upstream", Harness: "pi", Live: true,
			Cwd: linked, Remote: "y"},
		stubAgent{ID: "recado://host/id", Harness: "pi", Live: true,
			Cwd: linked},
		stubAgent{ID: "opencode-1", Harness: "opencode", Live: true,
			Cwd: linked},
		stubAgent{ID: "match", Harness: "claude", Live: true,
			Cwd: filepath.Join(link, "src")},
	))
	sendCase(t, p.worktrees[0], stub,
		"REVISA_CASE=picker",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_OFFERS=1",
		"REVISA_MARKED=1",
		"REVISA_ABSENT=gone,codex,elsewhere,upstream,recado://",
		"REVISA_SENT=match")
	equalStrings(t, "argv", wantArgv("match", "feat/x"),
		stubArgv(t, stub, 1))
	if got := stubAddr(t, stub, 1); got != "revisa" {
		t.Errorf("RECADO_ADDR %q, want revisa", got)
	}
}

func TestBufferSendFallsBackToTheManualPicker(t *testing.T) {
	nowhere := t.TempDir()
	cases := map[string][]stubAgent{
		"duplicate": {
			{ID: "one", Harness: "claude", Live: true, Cwd: "WORKTREE"},
			{ID: "two", Harness: "pi", Live: true, Cwd: "WORKTREE"},
		},
		"unknown cwd": {
			{ID: "one", Harness: "claude", Live: true},
		},
		"opencode cwd": {
			{ID: "one", Harness: "opencode", Live: true,
				Cwd: "WORKTREE"},
		},
		"outside git": {
			{ID: "one", Harness: "claude", Live: true, Cwd: nowhere},
		},
		"missing dir": {
			{ID: "one", Harness: "claude", Live: true,
				Cwd: filepath.Join(nowhere, "deleted")},
		},
	}
	for name, agents := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			for i := range agents {
				if agents[i].Cwd == "WORKTREE" {
					agents[i].Cwd = linked
				}
			}
			stub := recadoStub(t, agentsJSON(t, agents...))
			sendCase(t, linked, stub,
				"REVISA_CASE=picker",
				"REVISA_SETTING=recado",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_OFFERS=1",
				"REVISA_MARKED=0",
				"REVISA_SENT="+agents[0].ID)
			if calls := stubCalls(t, stub); calls != 1 {
				t.Fatalf("recado was called %d times", calls)
			}
			if got := stubArgv(t, stub, 1)[1]; got != agents[0].ID {
				t.Errorf("sent to %q, want %q", got, agents[0].ID)
			}
		})
	}
}

func TestBufferSendWithoutALiveAgent(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "gone", Harness: "claude", Cwd: linked},
		stubAgent{ID: "codex", Harness: "codex", Live: true, Cwd: linked},
	))
	sendCase(t, linked, stub,
		"REVISA_CASE=picker",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_OFFERS=0",
		"REVISA_MARKED=0",
		"REVISA_ERROR=no live agent to notify")
	assertNoSend(t, stub)
}

func TestBufferSendCancelSendsNothing(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "match", Harness: "claude", Live: true, Cwd: linked},
	))
	sendCase(t, linked, stub,
		"REVISA_CASE=cancel",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_MARKED=1",
		"REVISA_PICK=cancel")
	assertNoSend(t, stub)
}

func TestBufferSendReportsRecadoFailures(t *testing.T) {
	cases := map[string]struct {
		codes  []int
		stderr string
		sent   string
		want   string
	}{
		"succeeds": {codes: []int{0}, sent: "match"},
		"fails": {
			codes: []int{1}, stderr: "recado: agent is gone",
			want: "recado: agent is gone",
		},
		"retries the picker": {
			codes: []int{2, 0}, stderr: "recado: agent is busy",
			sent: "match",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			stub := recadoStub(t, agentsJSON(t,
				stubAgent{ID: "match", Harness: "claude", Live: true,
					Cwd: linked},
				stubAgent{ID: "other", Harness: "pi", Live: true},
			))
			stubCodes(t, stub, c.codes...)
			if c.stderr != "" {
				stubStderr(t, stub, 1, c.stderr)
			}
			offers := strconv.Itoa(len(c.codes))
			picks := strings.TrimSuffix(
				strings.Repeat("1,", len(c.codes)), ",")
			sendCase(t, linked, stub,
				"REVISA_CASE=picker",
				"REVISA_SETTING=recado",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_OFFERS="+offers,
				"REVISA_MARKED=1",
				"REVISA_PICK="+picks,
				"REVISA_SENT="+c.sent,
				"REVISA_ERROR="+c.want)
			if calls := stubCalls(t, stub); calls != len(c.codes) {
				t.Fatalf("recado was called %d times, want %d",
					calls, len(c.codes))
			}
			for call := 1; call <= len(c.codes); call++ {
				equalStrings(t, "argv", wantArgv("match", "feat/x"),
					stubArgv(t, stub, call))
			}
		})
	}
}

func TestBufferSendQuotesTheBranchInTheMessage(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	other := filepath.Join(t.TempDir(), "quoted")
	git(t, linked, "worktree", "add", "-q", "-b", "feat/o'brien", other)
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "match", Harness: "claude", Live: true, Cwd: other},
	))
	sendCase(t, p.worktrees[0], stub,
		"REVISA_CASE=picker",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(other, "src/a.lua"),
		"REVISA_OFFERS=1",
		"REVISA_MARKED=1",
		"REVISA_SENT=match")
	equalStrings(t, "argv", wantArgv("match", "feat/o'brien"),
		stubArgv(t, stub, 1))
}

func TestBufferSendUsesTheWorktreeOfTheCurrentBuffer(t *testing.T) {
	for _, kind := range []string{"argument", "changes", "file", "diff"} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			stub := recadoStub(t, agentsJSON(t,
				stubAgent{ID: "match", Harness: "claude", Live: true,
					Cwd: linked},
			))
			env := []string{
				"REVISA_CASE=" + kind,
				"REVISA_SETTING=recado",
				"REVISA_FILE=" + filepath.Join(linked, "src/a.lua"),
				"REVISA_TARGET=feat/x",
				"REVISA_OFFERS=1",
				"REVISA_MARKED=1",
				"REVISA_SENT=match",
			}
			sendCase(t, p.worktrees[0], stub, env...)
			equalStrings(t, "argv", wantArgv("match", "feat/x"),
				stubArgv(t, stub, 1))
		})
	}
}

func TestBufferSendRefusesDetachedWorktree(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	detached := filepath.Join(t.TempDir(), "detached")
	git(t, linked, "worktree", "add", "-q", "--detach", detached)
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "match", Harness: "claude", Live: true,
			Cwd: detached},
	))
	sendCase(t, p.worktrees[0], stub,
		"REVISA_CASE=detached",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(detached, "src/a.lua"),
		"REVISA_ERROR=is not on a branch")
	assertNoSend(t, stub)
}

// TestBufferSendRunsInTheExplicitWorktree pins the same rule for send: an
// explicit worktree that is a directory runs the CLI there, so a send
// from outside git still reads that worktree and messages its branch.
func TestBufferSendRunsInTheExplicitWorktree(t *testing.T) {
	_, linked := changesProject(t)
	outside := t.TempDir()
	writeFile(t, outside, "notes.txt", "outside\n")
	stub := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "match", Harness: "claude", Live: true, Cwd: linked},
	))
	sendCase(t, outside, stub,
		"REVISA_CASE=argument",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(outside, "notes.txt"),
		"REVISA_TARGET="+linked,
		"REVISA_OFFERS=1",
		"REVISA_MARKED=1",
		"REVISA_SENT=match")
	equalStrings(t, "argv", wantArgv("match", "feat/x"),
		stubArgv(t, stub, 1))
}
