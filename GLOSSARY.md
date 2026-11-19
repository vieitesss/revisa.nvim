# revisa.nvim

Review what agents changed in any worktree of a project, leave comments, and have agents act on them.

## Language

**Comment**:
A reviewer's remark on a range of lines in one version of a file, on either side.
_Avoid_: Annotation, note

**Anchor text**:
The exact lines a comment was made on, used to find those lines again after the file changes.
_Avoid_: Line text

**File version**:
One state of one file under review: the new side at the branch's current text, or a file on the old side at one commit. A comment belongs to one file version, and only a new-side one re-anchors.
_Avoid_: Revision, snapshot

**Resolved**:
The state of a comment an agent has finished with, carrying the agent's reply. A rejected fix gets a new comment.
_Avoid_: Done, closed

**Outdated**:
A condition of an open new-side comment whose anchor text can no longer be found. It is not a state of the comment, so the comment still needs handling.
_Avoid_: Stale, orphaned

**Old side**:
The file version at the base of the changes under review. A deleted line exists only here.
_Avoid_: Left side, before

**New side**:
The file version at the current text of the file in the worktree under review.
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

**Changes list**:
A worktree's branch, base and session changes, as one reviewer-facing list. The plugin holds one per changes buffer, as its changes session; revisa rebuilds it from git and never stores it.
_Avoid_: Diff, changeset

**Review target**:
What one editor view shows of a project: a changes list, or one file version in a worktree. It decides where comments are drawn and which file version a new comment is stored against.
_Avoid_: Context, buffer

**Recipient**:
A live agent that `:Revisa send` can tell that comments are waiting on a branch.
_Avoid_: Target, destination

**Notifier**:
A way of reaching recipients, such as Rex or Recado. Each finds its own recipients and carries the message to them.
_Avoid_: Carrier, channel, backend
