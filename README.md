# revisa.nvim

Review what agents changed across a project's worktrees, leave comments on
lines, and have agents act on them. The repository contains a Neovim plugin,
a standalone Go CLI, and an agent skill.

## Install with lazy.nvim

Go must be installed and available on `PATH`. Add this plugin spec:

```lua
{
    "vieitesss/revisa.nvim",
    build = "go build -o bin/revisa ./cmd/revisa && bin/revisa install",
}
```

The build step compiles the CLI into the checkout's `bin/revisa`, then runs
`revisa install`. You can run the same command manually from the checkout:

```sh
go build -o bin/revisa ./cmd/revisa && bin/revisa install
```

## What `revisa install` does

It creates two symlinks, making parent directories when needed:

- `~/.local/bin/revisa` points to the built `<checkout>/bin/revisa` binary.
- `~/.agents/skills/revisa` points to `<checkout>/skills/revisa`.

The checkout is found from the running binary's real path, even when you
run it through the installed link. Its `skills/revisa/SKILL.md` must exist.
Keep the checkout in place: the installed links depend on it.

To choose a different directory for the CLI link:

```sh
bin/revisa install --bin-dir "$HOME/bin"
```

`--bin-dir` only changes the CLI link. The skill always goes into
`~/.agents/skills`.

If the CLI directory is not on `PATH`, install prints a warning to stderr
but still succeeds. Add the directory to your shell configuration before
using `revisa` from a new shell. For the default directory:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Install prints one stdout line for each new or already-present link.
Running it twice is safe: matching links stay unchanged. It never replaces
an unrelated file, directory, or symlink. A failure names the path involved
and exits with code `1`; invalid arguments exit with code `2`.

## Review a worktree

From a shell in the project, `revisa review` opens Neovim in the worktree
picker, and `revisa review <worktree>` opens one worktree directly. A
worktree is a path or a branch. The working directory is kept, and
`$REVISA_NVIM` chooses another Neovim executable.

Inside Neovim, `:Revisa review [WORKTREE]` does the same. Without a
worktree it uses `vim.ui.select` over the project's worktrees, so no
picker plugin is needed.

The list shows that worktree's commits since the merge-base with the
project's default branch, plus uncommitted changes and untracked files.
On the default branch it shows only the uncommitted work, because
sessions do not commit there.

| Key    | Action                                                 |
| ------ | ------------------------------------------------------ |
| `<CR>` | Open the change as a diff: base left, file right        |
| `o`    | Open the file alone, in the previous window             |
| `R`    | Reload the list from the CLI                            |
| `q`    | Close the list                                          |

Comments on the opened file appear as usual.

The diff view opens in a new tab, with the content at the base on the
left and the real file on the right, both in Neovim's diff mode. The
right side stays editable, so comments work there like on any file. A
renamed entry reads its old path on the left, an added or untracked one
has an empty left side, and a deleted one has an empty right side.
Revisa computes no diff: git provides the base content and Neovim
renders the comparison. `q` on the left side closes the tab. A file
with a NUL byte in its first 8000 bytes is reported as binary instead
of opened.

The CLI exposes the same data for scripts and agents:

```sh
revisa worktrees [--json]
revisa changes [--worktree PATH|BRANCH] [--json]
```

`changes` prints a header line, `worktree <path> <branch> base <sha>
(<default branch>)`, then one `STATUS path` line per entry, with
`old -> new` after a rename. Status is `M`, `A`, `D`, `R` or `?` for an
untracked file.

## Tell the agent that comments are waiting

Agents can always pull comments with `revisa list`, so notification is
optional. To send one through Recado, turn it on once:

```lua
require("revisa").setup({ notify = "recado" })
```

`recado` must be on `PATH`, and `:Revisa send [WORKTREE]` then asks you
which live agent to message and sends it after you confirm. Without a
worktree it uses the changes buffer's, the diff view's, or the current
file's. Remote agents and Codex are not offered, since they cannot
receive these messages. The one agent working in that worktree is
offered first, marked as the match; anything else, or several matches,
leaves the choice to you.

The message carries no comment content, only where to read it: run
`revisa list --branch '<branch>' --status open` and answer each comment
with `revisa resolve`, not `recado reply`. Revisa reads no recado inbox
and registers no agent. A successful send prints `revisa: sent to <id>`
and returns; a failure prints recado's error; exit code `2` reopens the
picker. The `notify` setting is `"none"` by default.
