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

**Open**:
The state of a comment no agent has finished with yet.
_Avoid_: Pending, active

**Reply**:
The single answer an agent leaves on a comment when resolving it. There are no threads.
_Avoid_: Response, thread

**Unknown**:
A condition of an open new-side comment whose branch no longer exists, so whether it is outdated cannot be told.
_Avoid_: Missing, lost

**Project**:
All worktrees of one repository, or one directory outside git, which share one set of comments. In a repository, comments belong to a branch, not to a worktree; outside git they belong to no branch.
_Avoid_: Repo, workspace

**Store**:
Where a project keeps its comments, one file per comment, shared by all its worktrees: under the git common directory in a git repository, or `.revisa/` at the root of a plain directory. Agents and the plugin reach it only through the CLI.
_Avoid_: Database, cache

**Base**:
The commit where the branch under review left the project's default branch.
_Avoid_: Target, parent

**Session changes**:
Everything that differs between the base and a worktree, including new files git does not track yet.
_Avoid_: Diff, changeset
