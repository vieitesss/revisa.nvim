local buf = H.open()
if vim.env.REVISA_PLUGIN_LOAD == "late" then
    H.equal(nil, vim.g.loaded_revisa)
    vim.cmd.enew()
    local unnamed = vim.api.nvim_get_current_buf()
    local special = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(special, "special.txt")
    local unloaded = vim.fn.bufadd("unloaded.txt")
    local recorded = vim.api.nvim_create_buf(false, false)
    vim.api.nvim_buf_set_name(recorded, "recorded.txt")
    local saved = { name = "previous.txt", path = "preserved.txt" }
    vim.b[recorded].revisa_file = saved
    vim.cmd("runtime plugin/revisa.lua")
    H.equal(nil, vim.b[unnamed].revisa_file)
    H.equal(nil, vim.b[special].revisa_file)
    H.equal(nil, vim.b[unloaded].revisa_file)
    H.equal(saved, vim.b[recorded].revisa_file)
    vim.api.nvim_set_current_buf(buf)
    vim.cmd("Revisa refresh")
end
local absolute = vim.env.REVISA_ABSOLUTE
local target = require("revisa.target")
local buffer = require("revisa.buffer")
local found = target.of(buf)
H.equal(absolute, found.absolute)
H.equal(vim.env.REVISA_ROOT, found.cwd)
H.display(buf, 1, 1, { "Existing alias comment" })

-- A directory change rewrites Neovim's short name, not the file identity.
vim.cmd.cd("/")
H.equal(absolute, buffer.file(buf))
vim.cmd.cd(vim.fn.fnameescape(vim.env.REVISA_ROOT))
vim.cmd("edit!")
H.equal(absolute, target.of(buf).absolute)
H.display(buf, 1, 1, { "Existing alias comment" })
vim.cmd.enew()
H.equal(absolute, target.of(buf).absolute)
vim.api.nvim_set_current_buf(buf)

vim.cmd("2Revisa comment")
local editor = H.float()
vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "New alias comment" })
vim.cmd("write")
local drawn = require("revisa.display").at(buf, 2)
H.equal(1, #drawn)
H.equal("New alias comment", drawn[1].comment.body)

-- Renaming between aliases replaces the saved name, even for one file.
local directory = vim.fs.dirname(absolute)
local alias = vim.fs.dirname(directory) .. "/alias-b/real.txt"
local full_name = vim.api.nvim_buf_get_name(buf)
vim.cmd.file(vim.fn.fnameescape(alias))
H.equal(full_name, vim.api.nvim_buf_get_name(buf))
H.equal(alias, target.of(buf).absolute)
vim.cmd("Revisa refresh")
H.equal(0, #require("revisa.display").at(buf, 1))
vim.cmd("1Revisa comment")
editor = H.float()
vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "Renamed alias comment" })
vim.cmd("write")
H.display(buf, 1, 1, { "Renamed alias comment" })

-- A renamed buffer gets its new lexical name, not the saved old name.
local renamed = directory .. "/renamed.txt"
vim.cmd.file(vim.fn.fnameescape(renamed))
H.equal(renamed, target.of(buf).absolute)

-- A new file keeps its lexical name after it reaches disk.
local created = directory .. "/created.txt"
vim.cmd.edit(vim.fn.fnameescape(created))
vim.api.nvim_buf_set_lines(0, 0, -1, false, { "created" })
vim.cmd.write()
H.equal(created, target.of(vim.api.nvim_get_current_buf()).absolute)
