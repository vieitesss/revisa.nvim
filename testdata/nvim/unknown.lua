local api = vim.api
local buf = H.open()

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
