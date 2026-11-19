# The plugin hands the CLI absolute paths; only the CLI maps a file to its project and store path

Every review target carries the absolute path of its file: a pin builds it
from its worktree and stored path, a plain file buffer from the file itself.
The plugin passes it to `revisa add --path` and `revisa list --file`. Only
the CLI decides which project a file belongs to and what the store calls it:
the path relative to the worktree root in git, or to the directory holding
`.revisa/` outside git. JSON output keeps that relative path, so what agents
read does not change.

## Considered Options

- **A `revisa context` command** returning the worktree, the run directory
  and the relative path. Rejected when the plugin composed targets locally:
  it added a process to every refresh and a second JSON contract. Neither
  objection holds now. The plugin already runs one CLI command per refresh
  and already reads its JSON, so the mapping rides on the process and the
  contract that exist: no new process, no new JSON.
- **The same discovery in Lua**, with `git rev-parse --show-toplevel` and a
  walk up for `.revisa/`. Rejected: it was a second copy of the CLI's rules
  for the one thing the two must agree on, the stored path, so any drift
  filed a comment where no `revisa list` would look for it.
- **A relative path computed by the plugin and passed as today.** Rejected:
  it needs the same discovery to make it relative, and a file outside the
  project root cannot spell its path at all.

## Consequences

A plain file buffer's target has no worktree: the CLI runs in the editor's
working directory when that contains the file, else in the file's directory,
and finds the worktree from there. `:Revisa send` therefore runs
`changes --json` with no `--worktree` for such a buffer. A file whose lines
moved or were deleted is unaffected: the project-relative suffix is lexical,
so an old-side path that no longer exists still maps, and `--side old` reads
its content from `--commit`.

Only aliases of the project root and its ancestors are resolved, such as
macOS `/var` pointing to `/private/var`. Below that root, the path the user
gave is the stored identity, not the symlink's destination: `alias.txt`
remains `alias.txt` whether passed relative or absolute. This also applies
to symlinked directories inside the project, even when a link points outside
it. The plugin never resolves a file's symlinks before handing its absolute
path to the CLI.

The plugin runs no git command to place a comment, and the CLI refuses an
absolute path outside the project with exit code 1. The rules live in
`internal/project` and are pinned there; the headless tests check only that
the plugin hands over the absolute path it shows.
