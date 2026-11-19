# revisa.nvim

Review what agents changed across a project's worktrees, leave comments on
lines, and have agents act on them. The repository contains a Neovim plugin,
a standalone Go CLI, and an agent skill.

## Requirements

- Neovim 0.12 or newer.
- Go 1.27 or newer on `PATH`, to build the CLI.

## Install

Both plugin managers below build the CLI into the checkout's `bin/revisa`,
then run `revisa install`. It links the CLI to `~/.local/bin/revisa` and the
agent skill to `~/.agents/skills/revisa`. Keep `~/.local/bin` on `PATH`.
See `:help revisa-install` for details.

### vim.pack

Define the build hook before `vim.pack.add()`, so it also runs on the first
install:

```lua
vim.api.nvim_create_autocmd("PackChanged", {
    callback = function(ev)
        local kind = ev.data.kind
        if ev.data.spec.name ~= "revisa.nvim"
            or (kind ~= "install" and kind ~= "update") then
            return
        end
        local build = "go build -o bin/revisa ./cmd/revisa"
            .. " && bin/revisa install"
        local result = vim.system({ "sh", "-c", build },
            { cwd = ev.data.path, text = true }):wait()
        if result.code ~= 0 then
            vim.notify("revisa.nvim: build failed\n" .. result.stderr,
                vim.log.levels.ERROR)
        end
    end,
})

vim.pack.add({
    {
        src = "https://github.com/vieitesss/revisa.nvim",
        version = vim.version.range("*"),
    },
})
```

### lazy.nvim

```lua
{
    "vieitesss/revisa.nvim",
    version = "*",
    build = "go build -o bin/revisa ./cmd/revisa && bin/revisa install",
}
```

`version` follows the latest release instead of `main` in both managers.

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
