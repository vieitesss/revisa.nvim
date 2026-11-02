# revisa.nvim

Review what agents changed in any worktree of a project, leave comments, and have agents act on them.

## Language

**Comment**:
A reviewer's remark on a range of lines in one version of a file, on either side.
_Avoid_: Annotation, note

**Anchor text**:
The exact lines a comment was made on, used to find those lines again after the file changes.
_Avoid_: Line text

**Resolved**:
The state of a comment an agent has finished with, carrying the agent's reply. A rejected fix gets a new comment.
_Avoid_: Done, closed

**Outdated**:
A condition of an open new-side comment whose anchor text can no longer be found. It is not a state of the comment, so the comment still needs handling.
_Avoid_: Stale, orphaned

**Old side**:
The version of a file at the base of the changes under review. A deleted line exists only here.
_Avoid_: Left side, before

**New side**:
The current version of a file in the worktree under review.
_Avoid_: Right side, after
