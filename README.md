# revisa.nvim

Review what agents changed across a project's worktrees, leave comments on
lines, and have agents act on them. The repository contains a Neovim plugin,
a standalone Go CLI, and an agent skill.

## Requirements

- Neovim 0.12 or newer.
- Go 1.27 or newer on `PATH`, to build the CLI.

## Install

With `vim.pack`:

```lua
vim.pack.add({
    {
        src = "https://github.com/vieitesss/revisa.nvim",
        version = vim.version.range("*"),
    },
})
```

With lazy.nvim:

```lua
{ "vieitesss/revisa.nvim", version = "*" }
```

`version` follows the latest release instead of `main`.

The plugin builds its own CLI. On the first start after an install or
update, it runs `go build` in the background, then `revisa install`. That
links the CLI to `~/.local/bin/revisa` and the agent skill to
`~/.agents/skills/revisa`; keep `~/.local/bin` on `PATH`. To build and
install the CLI yourself, set `vim.g.revisa_auto_build = false` before the
plugin loads. See `:help revisa-install`.

## Quick start

1. Review a worktree: run `revisa review` in a shell, or `:Revisa review`
   in Neovim. Pick a worktree to list its changes, then press `<CR>` on a
   file to open it as a diff against its base.
2. Comment: `:Revisa comment` on a line, or `:'<,'>Revisa comment` on a
   selection. Write the body and save it with `:w`.
3. Tell the agent: `:Revisa send` points an agent at the waiting comments.
   It works through Recado or Rex, set with
   `require("revisa").setup({ notify = "recado" })` or `"rex"`.
4. The agent reads the comments with `revisa list` and answers each one
   with `revisa resolve`, following the installed skill. `:Revisa resolved`
   shows its replies.

There are no default keymaps. For example:

```lua
vim.keymap.set("n", "<leader>rc", "<Plug>(revisa-comment)")
vim.keymap.set("x", "<leader>rc", "<Plug>(revisa-comment)")
vim.keymap.set("n", "<leader>rr", "<cmd>Revisa review<cr>")
```

## Documentation

- `:help revisa` for the plugin: commands, diff view, display and setup.
- [`skills/revisa/SKILL.md`](skills/revisa/SKILL.md) for the CLI as agents
  use it.
- [`GLOSSARY.md`](GLOSSARY.md) for the terms revisa uses.

## License

[MIT](LICENSE)
