local api = vim.api
vim.o.columns, vim.o.lines = 120, 30
local buf = H.open()
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(buf))

api.nvim_buf_set_lines(buf, 0, 0, false, { "one", "two", "three" })
api.nvim_buf_set_lines(buf, 4, -1, false, {})
H.equal(4, api.nvim_buf_line_count(buf))

vim.v.errmsg = ""
vim.cmd("vertical resize 40")
return function()
    H.equal("", vim.v.errmsg)
    H.equal({}, H.signs(buf))
    H.equal({}, H.virtual(buf))
    vim.v.errmsg = ""
    vim.cmd("vertical resize 50")
    return function()
        H.equal("", vim.v.errmsg)
        H.equal({}, H.signs(buf))
        H.equal({}, H.virtual(buf))
    end
end
