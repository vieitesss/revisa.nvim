# Outdated comments render in an end-of-buffer block

An outdated comment has no current lines: its anchor text is gone, so any
line is a guess. The plugin shows outdated comments only in one virtual
block below the last line, headed `outdated (N)`, each entry with its body
and its `was:` lines. `was:` uses `RevisaWas`, linked to `Comment`. While the
block is non-empty, a right-aligned virtual text on line 1 reads
`revisa: 2 outdated` in `RevisaOutdated`, so the block is findable in a long
file. A comment never gets a sign or extmark on the stored line. The block
disappears on the next refresh when the text comes back, and the comment
returns to its line.

Unknown remains a CLI condition for agents passing `--branch`: the branch
no longer exists, so the CLI cannot tell whether its comments are outdated.
The plugin lists only the worktree's checked-out branch, which git cannot
delete while it is checked out. The plugin therefore cannot receive
unknown comments and does not render them.

## Considered Options

- **A sign on the stored line.** Rejected: the line is a guess, which the
  outdated marker exists to avoid.
- **A sign at end of file.** Rejected: it is not line-accurate, carries no
  body, and is indistinguishable from a file-level marker.
- **A floating window.** Rejected: it is not part of the buffer and
  buffer-text queries do not find it.
- **One line per comment.** Rejected: it loses the body and the `was:`
  context.

## Consequences

A reviewer sees the comment without seeing the code; the `was:` lines
carry the context. The CLI keeps its `was:` output for agents. The block is
virtual lines, so it does not modify the buffer, does not affect `:write`,
and is found by buffer-text queries only through the extmark namespace.
Groups: `RevisaOutdated` (link `DiagnosticWarn`), `RevisaWas` (link
`Comment`), body `RevisaComment`.
