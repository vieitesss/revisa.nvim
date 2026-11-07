local buf, win = H.open()
vim.wo.foldmethod = "manual"
vim.cmd("2,10fold")
H.equal(2, vim.fn.foldclosed(6))
vim.cmd("4,6Revisa comment")
local _, float = H.float()
vim.cmd.redraw()
local top = vim.fn.win_screenpos(float)[1]
local bottom = top + vim.api.nvim_win_get_height(float)
for line = 1, vim.api.nvim_buf_line_count(buf) do
    local row = vim.fn.screenpos(win, line, 1).row
    assert(row == 0 or row < top or row >= bottom,
        "float covers file line " .. line .. " at screen row " .. row)
end
-- Neovim expands an Ex range inside a closed fold to its boundaries.
H.reserved(buf, win, 10)
H.keys("q")
assert(not H.padding(buf), "padding remains after cancel")
H.equal(win, vim.api.nvim_get_current_win())
