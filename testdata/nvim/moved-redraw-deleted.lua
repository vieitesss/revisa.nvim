-- A comment whose anchor lines are deleted is hidden by a resize redraw:
-- never drawn on a guessed line, never past the end of the buffer.
local api = vim.api
vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(buf))

-- Three lines go in at the top, then the anchor and the file's tail go
-- away, so the sign collapses to the end of a four-line buffer.
api.nvim_buf_set_lines(buf, 0, 0, false, { "one", "two", "three" })
api.nvim_buf_set_lines(buf, 4, -1, false, {})
H.equal(4, api.nvim_buf_line_count(buf))

vim.v.errmsg = ""
vim.cmd("vertical resize 40")
-- The resize redraw runs when the window change settles.
return function()
    H.equal("", vim.v.errmsg)
    H.equal({}, H.signs(buf))
    H.equal({}, H.virtual(buf))
    -- A second redraw keeps it hidden instead of drawing it from the
    -- CLI's old coordinates.
    vim.v.errmsg = ""
    vim.cmd("vertical resize 50")
    return function()
        H.equal("", vim.v.errmsg)
        H.equal({}, H.signs(buf))
        H.equal({}, H.virtual(buf))
    end
end
