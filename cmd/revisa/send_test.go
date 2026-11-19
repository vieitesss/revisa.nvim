package main_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
	State   string `json:"state,omitempty"`
	Cwd     string `json:"cwd,omitempty"`
	Machine string `json:"machine,omitempty"`
	Remote  string `json:"remote,omitempty"`
}

func agentsJSON(t *testing.T, agents ...stubAgent) string {
	t.Helper()
	return jsonList(t, "agents", agents)
}

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

func stubArgv(t *testing.T, stub string, call int) []string {
	t.Helper()
	data, err := os.ReadFile(
		filepath.Join(stub, "send"+strconv.Itoa(call)))
	if err != nil {
		t.Fatalf("no recorded send call %d: %v", call, err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

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

const reviewMessage = "revisa: review comments are waiting on branch " +
	"'feat/x'. Run: revisa list --branch 'feat/x' --status open." +
	" Answer each with revisa resolve."

func wantArgv(id string) []string {
	return []string{
		"send", id, reviewMessage, "--meta", "revisa_branch=feat/x",
	}
}

func sendCase(t *testing.T, dir, stub string, env ...string) {
	t.Helper()
	headless(t, dir, "send.lua", append([]string{
		"RECADO_STUB=" + stub,
		"PATH=" + notifierPath(t, stub),
		"REX_SESSION=",
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
			want:    "Revisa send is off",
		},
		"recado missing": {
			setting: "recado",
			want:    "recado is not on PATH",
		},
		"auto without a notifier": {
			want: "needs rex, with REX_SESSION set and rex on PATH," +
				" or recado on PATH",
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
				"PATH="+notifierPath(t, path))
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
		"REVISA_ERROR=auto, none, recado or rex")
	assertNoSend(t, stub)
}

func TestBufferSendListsRecipientsInOrder(t *testing.T) {
	p, home := homeProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	outside := filepath.Join(home, "café")
	if err := os.MkdirAll(outside, 0o755); err != nil {
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
		stubAgent{ID: "far", Harness: "aider", Live: true},
		stubAgent{ID: "main", Harness: "claude", Live: true, Cwd: main,
			State: "busy"},
		stubAgent{ID: "linked", Harness: "claude", Live: true,
			Cwd: filepath.Join(linked, "src"), State: "idle"},
		stubAgent{ID: "plain", Harness: "pi", Live: true, Cwd: outside,
			State: "idle"},
		stubAgent{ID: "opencode", Harness: "opencode", Live: true,
			Cwd: linked, State: "busy"},
	))
	sendCase(t, linked, stub,
		"HOME="+home,
		"REVISA_CASE=picker",
		"REVISA_SETTING=recado",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_OFFERS=1",
		"REVISA_PANES="+strings.Join([]string{
			"claude    ~/w/linked/src  feat/x  idle",
			"claude    ~/w/main        main    busy",
			"aider",
			"pi        ~/café                  idle",
			"opencode                          busy",
		}, "\n"),
		"REVISA_ABSENT=gone",
		"REVISA_SENT=claude in ~/w/linked/src")
	equalStrings(t, "argv", wantArgv("linked"), stubArgv(t, stub, 1))
	if got := stubAddr(t, stub, 1); got != "revisa" {
		t.Errorf("RECADO_ADDR %q, want revisa", got)
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
		"REVISA_ERROR=no recipient to notify")
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
		"fails": {
			codes: []int{1}, stderr: "recado: agent is gone",
			want: "recado: agent is gone",
		},
		"retries the picker": {
			codes: []int{2, 0}, stderr: "recado: agent is busy",
			sent: "claude in ~/w/linked",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p, home := homeProject(t)
			linked := p.worktrees[1]
			writeFile(t, linked, "src/a.lua", "anchor\n")
			stub := recadoStub(t, agentsJSON(t,
				stubAgent{ID: "match", Harness: "claude", Live: true,
					Cwd: linked},
			))
			stubCodes(t, stub, c.codes...)
			if c.stderr != "" {
				stubStderr(t, stub, 1, c.stderr)
			}
			offers := strconv.Itoa(len(c.codes))
			picks := strings.TrimSuffix(
				strings.Repeat("1,", len(c.codes)), ",")
			sendCase(t, linked, stub,
				"HOME="+home,
				"REVISA_CASE=picker",
				"REVISA_SETTING=recado",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_OFFERS="+offers,
				"REVISA_PANES=claude  ~/w/linked  feat/x",
				"REVISA_PICK="+picks,
				"REVISA_SENT="+c.sent,
				"REVISA_ERROR="+c.want)
			if calls := stubCalls(t, stub); calls != len(c.codes) {
				t.Fatalf("recado was called %d times, want %d",
					calls, len(c.codes))
			}
			for call := 1; call <= len(c.codes); call++ {
				equalStrings(t, "argv", wantArgv("match"),
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
		"REVISA_PANES=claude  "+other+"  feat/o'brien",
		"REVISA_SENT=claude in "+other)
	want := []string{
		"send", "match",
		`revisa: review comments are waiting on branch 'feat/o'\''brien'.` +
			` Run: revisa list --branch 'feat/o'\''brien' --status open.` +
			` Answer each with revisa resolve.`,
		"--meta", "revisa_branch=feat/o'brien",
	}
	equalStrings(t, "argv", want, stubArgv(t, stub, 1))
}

func TestBufferSendChoosesTheWorktreeUnderReview(t *testing.T) {
	for _, kind := range []string{"argument", "changes", "diff"} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			stub := recadoStub(t, agentsJSON(t,
				stubAgent{ID: "match", Harness: "claude", Live: true,
					Cwd: linked},
			))
			file := filepath.Join(linked, "src/a.lua")
			if kind == "argument" {
				file = filepath.Join(p.worktrees[0], "src/a.lua")
			}
			env := []string{
				"REVISA_CASE=" + kind,
				"REVISA_SETTING=recado",
				"REVISA_FILE=" + file,
				"REVISA_TARGET=feat/x",
				"REVISA_OFFERS=1",
				"REVISA_PANES=claude  " + linked + "  feat/x",
				"REVISA_SENT=claude in " + linked,
			}
			sendCase(t, p.worktrees[0], stub, env...)
			equalStrings(t, "argv", wantArgv("match"),
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
		"REVISA_PANES=claude  "+linked+"  feat/x",
		"REVISA_SENT=claude in "+linked)
	equalStrings(t, "argv", wantArgv("match"),
		stubArgv(t, stub, 1))
}

func TestBufferSendFallsBackToRecado(t *testing.T) {
	cases := map[string]string{
		"outside Rex": "",
		"rex missing": "session:one",
	}
	for name, session := range cases {
		t.Run(name, func(t *testing.T) {
			p, home := homeProject(t)
			linked := p.worktrees[1]
			writeFile(t, linked, "src/a.lua", "anchor\n")
			stub := recadoStub(t, agentsJSON(t,
				stubAgent{ID: "match", Harness: "claude", Live: true,
					Cwd: linked},
			))
			sendCase(t, linked, stub,
				"HOME="+home,
				"REX_SESSION="+session,
				"REVISA_CASE=picker",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_SETTING=",
				"REVISA_OFFERS=1",
				"REVISA_PANES=claude  ~/w/linked  feat/x",
				"REVISA_SENT=claude in ~/w/linked")
			equalStrings(t, "argv", wantArgv("match"), stubArgv(t, stub, 1))
		})
	}
}
