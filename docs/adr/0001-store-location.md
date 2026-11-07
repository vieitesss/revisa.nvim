# Comments stay under the git common directory; `.revisa/` only outside git

Inside a git repository, comments live in `<git common dir>/revisa/`. A `.revisa/` directory at the project root is used only where there is no git repository. We considered `.revisa/` as the only store, so users could choose to commit their comments, and rejected it.

## Considered Options

- **One `.revisa/` at the main checkout root.** Claude Code's sandbox lets an agent in a linked worktree write to the git common directory but not to the main checkout's working tree, so `revisa resolve` would fail there. Committing it would also put every branch's comments on whichever branch the main checkout has.
- **One `.revisa/` per worktree, committed with its branch.** Uncommitted comments would be lost when a worktree is removed, and the reviewer would have to collect comments from every worktree.

## Consequences

When a directory holding `.revisa/` becomes part of a git repository, `revisa list` warns that those comments are still there. `revisa migrate` moves them into the git store, stamped with the current branch or the one `--branch` names. Nothing moves them automatically.
