local buf, win = H.open()
local visual = vim.env.REVISA_MODE == "visual"
local last = visual and 4 or 2

if not visual then
    vim.cmd("2Revisa comment")
    local editor = H.reserved(buf, win, 2)
    vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "Discard me" })
    H.keys("q")
    assert(not H.padding(buf), "padding remains after cancel")
    vim.cmd("2Revisa comment")
    editor = H.reserved(buf, win, 2)
    vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "  ", "\t" })
    vim.cmd("write")
    assert(not H.padding(buf), "padding remains after empty save")
end

if visual then
    H.keys("2Gv4G<Plug>(revisa-comment)")
else
    vim.api.nvim_win_set_cursor(win, { 2, 0 })
    H.keys("<Plug>(revisa-comment)")
end
local editor = H.reserved(buf, win, last)
vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "Saved with Ctrl-s" })
H.keys((visual and "i" or "") .. "<C-s>")
assert(not H.padding(buf), "padding remains after Ctrl-s")
H.display(buf, 2, last, { "Saved with Ctrl-s" })
