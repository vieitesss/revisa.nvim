local messages = H.notifications()
local buf, win = H.open()
vim.cmd("2Revisa comment")
local editor = H.reserved(buf, win, 2)
vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "Fallback binary" })
vim.cmd("write")
if vim.env.REVISA_CASE == "missing" then
    H.error(messages, "revisa on PATH")
    H.error(messages, vim.env.REVISA_PLUGIN .. "/bin/revisa")
    H.equal({ "Fallback binary" },
        vim.api.nvim_buf_get_lines(editor, 0, -1, false))
    H.reserved(buf, win, 2)
    H.keys("q")
    assert(not H.padding(buf), "padding remains after cancel")
else
    H.equal({}, messages)
    H.display(buf, 2, 2, { "Fallback binary" })
end
