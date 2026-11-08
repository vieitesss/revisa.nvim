local api = vim.api
local buf = H.open()
vim.cmd("Revisa refresh")

-- The anchor text is gone: the comment is a marker, not a sign.
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

-- The anchor text comes back, as a fix would put it back.
api.nvim_buf_set_lines(buf, 0, -1, false,
    { "local a = 1", "local x = y", "if x then", "  run(x)", "end" })
vim.cmd.write()
vim.cmd("Revisa refresh")

-- The comment is back on its line and the block is gone.
H.equal({ { line = 5, text = ">", group = "RevisaSign" } }, H.signs(buf))
H.equal({
    {
        row = 5,
        above = true,
        lines = { { { "This text is gone", "RevisaComment" } } },
    },
}, H.virtual(buf))
H.equal(nil, H.text(buf))

-- The rendered screen agrees: no block and no summary any more.
vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
H.equal(nil, H.row(rows, "outdated ("))
H.equal(nil, H.row(rows, "revisa:"))
