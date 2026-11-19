local buf = H.open()
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

-- A renamed buffer gets its new lexical name, not the saved old name.
local directory = vim.fs.dirname(absolute)
local renamed = directory .. "/renamed.txt"
vim.cmd.file(vim.fn.fnameescape(renamed))
H.equal(renamed, target.of(buf).absolute)

-- A new file keeps its lexical name after it reaches disk.
local created = directory .. "/created.txt"
vim.cmd.edit(vim.fn.fnameescape(created))
vim.api.nvim_buf_set_lines(0, 0, -1, false, { "created" })
vim.cmd.write()
H.equal(created, target.of(vim.api.nvim_get_current_buf()).absolute)
