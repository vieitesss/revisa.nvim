-- Deleting lines above a comment keeps it on the line its sign moved to,
-- even though the CLI's last coordinates are past the buffer end.
local api = vim.api
vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 5, text = ">", group = "RevisaSign" } }, H.signs(buf))

-- Two lines above the anchor go away with no refresh: 3 lines remain, and
-- the sign is on the last of them.
api.nvim_buf_set_lines(buf, 0, 2, false, {})
H.equal(3, api.nvim_buf_line_count(buf))
H.equal({ { line = 3, text = ">", group = "RevisaSign" } }, H.signs(buf))

vim.cmd("vertical resize 40")
-- The resize redraw runs when the window change settles.
return function()
    H.display(buf, 3, 3, { "Shrunk" })
end
