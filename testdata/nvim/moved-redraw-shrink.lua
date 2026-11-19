local api = vim.api
vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 5, text = ">", group = "RevisaSign" } }, H.signs(buf))

api.nvim_buf_set_lines(buf, 0, 2, false, {})
H.equal(3, api.nvim_buf_line_count(buf))
H.equal({ { line = 3, text = ">", group = "RevisaSign" } }, H.signs(buf))

vim.cmd("vertical resize 40")
return function()
    H.display(buf, 3, 3, { "Shrunk" })
end
