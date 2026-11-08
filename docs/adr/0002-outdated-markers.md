# Outdated and unknown comments render in an end-of-buffer block

An outdated or unknown comment has no current lines: its anchor text is gone, so any line is a guess. The buffer shows it in one virtual block below the last line, grouped by condition (`outdated (N)`, `unknown (N)`), each entry with its body and its `was:` lines. `was:` uses the new `RevisaWas` group, linked to `Comment`. While the block is non-empty, a right-aligned virtual text on line 1 reads `revisa: 2 outdated, 1 unknown` in `RevisaOutdated`, so the block is findable in a long file. A comment never gets a sign or extmark on the stored line. The block disappears on the next refresh when the text comes back, and the comment returns to its line.

## Considered Options

- **A sign on the stored line.** Rejected: the line is a guess, which the outdated marker exists to avoid.
- **A sign at end of file.** Rejected: it is not line-accurate, carries no body, and is indistinguishable from a file-level marker.
- **A floating window.** Rejected: it is not part of the buffer and buffer-text queries do not find it.
- **One line per comment.** Rejected: it loses the body and the `was:` context.

## Consequences

A reviewer sees the comment without seeing the code; the `was:` lines carry the context. The CLI keeps its `was:` output for agents. The block is virtual lines, so it does not modify the buffer, does not affect `:write`, and is found by buffer-text queries only through the extmark namespace. Groups: `RevisaOutdated` (link `DiagnosticWarn`), `RevisaUnknown` (link `DiagnosticError`), `RevisaWas` (link `Comment`), body `RevisaComment`.
