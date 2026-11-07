local buf = H.open()
vim.api.nvim_win_set_cursor(0, { 2, 0 })
vim.cmd("Revisa comment")
local float = H.float()
vim.api.nvim_buf_set_lines(float, 0, -1, false, {
    "Handle nil", "Keep the context.",
})
vim.cmd("write")
H.display(buf, 2, 2, { "Handle nil", "Keep the context." })
