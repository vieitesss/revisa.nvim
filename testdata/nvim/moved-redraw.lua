vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(buf))

vim.api.nvim_buf_set_lines(buf, 0, 0, false, { "inserted" })
H.equal({ { line = 3, text = ">", group = "RevisaSign" } }, H.signs(buf))

vim.cmd("vertical resize 40")
return function()
    H.display(buf, 3, 3, { "Shifted" })
end
