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

   The header holds the id, the path from the repo root, the lines, the
   side and the commit the comment was made on.

2. **Read the code.**
   - `new` side: the lines are in the file as it is now in your worktree.
   - `old` side: the lines are gone or changed. Read them with
     `git show <commit>:<path>`.
   - A header ending in `outdated` has no line number: its lines changed
     after the comment. The `was:` lines show the text it was on. Find
     where that code went now.

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
