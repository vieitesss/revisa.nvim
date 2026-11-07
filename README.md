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
