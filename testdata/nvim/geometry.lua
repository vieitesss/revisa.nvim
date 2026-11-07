local buf, win = H.open()
if vim.env.REVISA_CASE == "diff" then
    vim.cmd("vnew")
    vim.bo.buftype = "nofile"
    vim.api.nvim_buf_set_lines(0, 0, -1, false, {
        "old-side scratch", "a line of text",
    })
    vim.cmd("diffthis")
    vim.api.nvim_set_current_win(win)
    vim.cmd("diffthis")
    vim.wo.foldenable = false
end
local last = vim.api.nvim_buf_line_count(buf)
if vim.env.REVISA_CASE == "scroll" then
    last = 20
end
vim.api.nvim_win_set_cursor(win, { last, 0 })
vim.cmd("normal! zb")
vim.cmd((last - 1) .. "," .. last .. "Revisa comment")
local _, editor_win = H.reserved(buf, win, last)
H.equal(false, vim.wo[editor_win].diff)
H.equal(false, vim.wo[editor_win].scrollbind)
if vim.env.REVISA_CASE == "diff" then
    vim.api.nvim_buf_set_lines(0, 0, -1, false, { "Diff tool comment" })
    vim.cmd("write")
    assert(not H.padding(buf), "padding remains after save")
    H.display(buf, last - 1, last, { "Diff tool comment" })
    return
end
if vim.env.REVISA_CASE == "scroll" then
    local _, float = H.float()
    vim.api.nvim_set_current_win(win)
    local keys = vim.api.nvim_replace_termcodes("5<C-y>", true, false, true)
    vim.api.nvim_feedkeys(keys, "mt", false)
    return function()
        vim.api.nvim_set_current_win(float)
        H.reserved(buf, win, last)
        H.keys("q")
        assert(not H.padding(buf), "padding remains after cancel")
    end
end
H.keys("q")
assert(not H.padding(buf), "padding remains after cancel")
H.equal(win, vim.api.nvim_get_current_win())
