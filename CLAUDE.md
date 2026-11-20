# revisa.nvim

A Neovim plugin (`lua/`, `plugin/`, `doc/`), a Go CLI (`cmd/revisa`,
`internal/`) and an agent skill (`skills/revisa`).

- Follow `CODING_STANDARDS.md`.
- Use the terms in `GLOSSARY.md`. Decisions live in `docs/adr/`.
- `just check` runs mechanical checks (`gofmt`, `go vet`, `errors.AsType`,
  80-column limit).
- `just test-fast` runs Go unit tests (`./internal/...`); `just test` runs
  every test, Go and headless Neovim (`cmd/revisa`, ~70 s). `just build`
  builds `bin/revisa`.
- Commits follow Conventional Commits. Mark breaking changes with `!`.

## Releases

Versions are `vX.Y.Z` tags with a GitHub release. A version covers the CLI
commands, flags, `--json` output and exit codes, the `:Revisa` commands and
`setup()` options, the agent skill, and the store's file format.

To cut a release:

1. Find the last release: `git describe --tags --abbrev=0`. With no tag,
   this is the first release.
2. Read every commit since it: `git log --oneline <last-tag>..origin/main`.
3. Suggest the next version. While the major is 0:
   - a breaking change (`!` or `BREAKING CHANGE`), or any `feat`, bumps
     the minor and resets the patch;
   - otherwise (`fix`, `refactor`, `docs`, ...) bumps the patch.

   Call out any change to the store's file format or `--json` output, even
   when it is not marked breaking.
4. Draft the release notes in a file outside the repository: what users
   gain, grouped as features, fixes and breaking changes, each breaking
   change with what users must do. Summarise; do not paste the commit log.
5. Show the user the version, the reasons for it and the notes. Wait for
   their approval. Never tag without it.
6. Run `just release <X.Y.Z> <notes-file>`, without the `v`. It checks
   that `main` is clean and equal to `origin/main`, pushes the tag and
   creates the GitHub release (run `just check` and `just test` before
   releasing).
