-- Rewrapping on a window resize keeps a comment on the line its extmark
-- moved to, instead of putting it back where the CLI last saw it.
vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(buf))

-- A line above moves the anchor: the comment is on line 3, unrefreshed.
vim.api.nvim_buf_set_lines(buf, 0, 0, false, { "inserted" })
H.equal({ { line = 3, text = ">", group = "RevisaSign" } }, H.signs(buf))

vim.cmd("vertical resize 40")
-- The resize redraw runs when the window change settles.
return function()
    H.display(buf, 3, 3, { "Shifted" })
end
