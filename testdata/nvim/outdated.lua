local buf = H.open()
vim.cmd("Revisa refresh")

-- The comment whose anchor text is gone keeps no sign on its stored line,
-- and its stored line is not in the buffer any more.
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(buf))

H.equal({
    {
        row = 2,
        above = true,
        lines = { { { "Kept", "RevisaComment" } } },
    },
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

-- The block is rendered below the last buffer line, not just stored, and
-- the summary is rendered flush right on the first line.
vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
local last = H.row(rows, "  run(x)")
local heading = H.row(rows, "outdated (1)")
assert(last and heading and heading > last,
    "block not rendered below the last line: " .. vim.inspect(rows))
assert(rows[heading + 1]:find("This text is gone", 1, true),
    "body not rendered: " .. vim.inspect(rows))
assert(rows[heading + 2]:find("was: end", 1, true),
    "was: not rendered: " .. vim.inspect(rows))
H.equal(1, H.row(rows, "revisa: 1 outdated"))
assert(#rows[1] > vim.o.columns - 3,
    "summary not right-aligned: " .. vim.inspect(rows[1]))
