local api = vim.api
local buf = H.open()
vim.cmd("Revisa refresh")

H.equal({}, H.signs(buf))
H.equal({
    {
        row = 4,
        above = true,
        lines = {
            { { "outdated (1)", "RevisaOutdated" } },
            { { "This text is gone", "RevisaComment" } },
            { { "was: end", "RevisaWas" } },
        },
    },
}, H.virtual(buf))
H.equal({ { "revisa: 1 outdated", "RevisaOutdated" } }, H.text(buf))

api.nvim_buf_set_lines(buf, 0, -1, false,
    { "local a = 1", "local x = y", "if x then", "  run(x)", "end" })
vim.cmd.write()
vim.cmd("Revisa refresh")

H.display(buf, 5, 5, { "This text is gone" })
H.equal(nil, H.text(buf))

vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
H.equal(nil, H.row(rows, "outdated ("))
H.equal(nil, H.row(rows, "revisa:"))
