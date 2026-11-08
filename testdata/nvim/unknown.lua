-- A comment the CLI reports as unknown is drawn as a marker, not a line.
local api = vim.api

local buf = api.nvim_create_buf(false, true)
api.nvim_buf_set_lines(buf, 0, -1, false, { "one", "two" })
local target = { kind = "file", path = "a.txt", side = "new" }

require("revisa.display").draw(buf, target, {
    {
        path = "a.txt",
        side = "new",
        status = "open",
        condition = "unknown",
        line_start = nil,
        line_end = nil,
        anchor_text = "one\ntwo",
        body = "The branch is gone",
    },
    {
        path = "a.txt",
        side = "new",
        status = "open",
        condition = "outdated",
        line_start = nil,
        line_end = nil,
        anchor_text = "three",
        body = "Gone line",
    },
}, false)

H.equal({}, H.signs(buf))
H.equal({
    {
        row = 2,
        above = true,
        lines = {
            { { "outdated (1)", "RevisaOutdated" } },
            { { "Gone line", "RevisaComment" } },
            { { "was: three", "RevisaWas" } },
            { { "", "RevisaComment" } },
            { { "unknown (1)", "RevisaUnknown" } },
            { { "The branch is gone", "RevisaComment" } },
            { { "was: one", "RevisaWas" } },
            { { "was: two", "RevisaWas" } },
        },
    },
}, H.virtual(buf))
H.equal({ { "revisa: 1 outdated, 1 unknown", "RevisaOutdated" } },
    H.text(buf))

-- The groups are rendered on screen, in order, with the summary on line 1.
api.nvim_win_set_buf(0, buf)
vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
H.equal(3, H.row(rows, "outdated (1)"))
H.equal(4, H.row(rows, "Gone line"))
H.equal(5, H.row(rows, "was: three"))
H.equal(7, H.row(rows, "unknown (1)"))
H.equal(8, H.row(rows, "The branch is gone"))
H.equal(9, H.row(rows, "was: one"))
H.equal(10, H.row(rows, "was: two"))
H.equal(1, H.row(rows, "revisa: 1 outdated, 1 unknown"))
assert(#rows[1] > vim.o.columns - 3,
    "summary not right-aligned: " .. vim.inspect(rows[1]))
