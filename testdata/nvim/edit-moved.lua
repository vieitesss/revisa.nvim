local api = vim.api
local buf = H.open()

-- the comments move down with the text they mark
api.nvim_buf_set_lines(buf, 0, 0, false, { "local preamble = 1" })
vim.cmd("write")
api.nvim_win_set_cursor(0, { 4, 0 })

local select = vim.ui.select
vim.ui.select = function(items)
    error("offered " .. #items .. " comments instead of the drawn one")
end
vim.cmd("Revisa edit")
vim.ui.select = select

local editor = H.float()
H.equal({ "Second" }, api.nvim_buf_get_lines(editor, 0, -1, false))
vim.cmd("normal q")
