---
name: revisa
description: >
  Act on code review comments a reviewer left with revisa, and resolve each
  with a reply. Use when told to address review comments, check revisa, or
  run `revisa list`.
---

# revisa

A reviewer leaves comments on lines of your branch. You fix what each
comment asks, then resolve it with one reply. The `revisa` CLI is the only
way to read and change comments.

## Steps

1. **List.** Run `revisa list --status open`: it prints the open comments
   on the current branch. Each comment prints as a header line, then its
   body:

   ```text
   01JC8Z7K3M9QX2V5T1R4N6P8WY src/a.lua:12-14 new 3f9c2e1
   Handle nil here
   ```

   The header holds the id, the path from the repo root, the original
   line range, the side and the commit the comment was made on.

2. **Read the code.**
   - `new` side: inspect the file in your worktree. The original lines may
     have moved or changed since the comment was made.
   - `old` side: read the commented version with
     `git show <commit>:<path>`.
   - Revisa does not yet detect outdated comments. If you cannot find the
     commented code, run `revisa list --status open --json` and use the
     comment's `anchor_text` to locate it.

3. **Fix.** Make the change the comment asks for, and check it the way you
   check any change in this repo (tests, lint).

   When a comment is unclear, or you think it is wrong, ask the user in
   this conversation. Keep that comment open until they answer.

4. **Resolve.** Run `revisa resolve <id> --reply "<what you changed>"`.
   The reply is the one answer the reviewer reads: say what changed and
   where, in a sentence or two. Resolve each comment once, after its fix
   is done.

Done when `revisa list --status open` prints only the comments you
asked the user about.
