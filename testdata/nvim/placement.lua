-- The placement table: where one comment goes in one review target.
local placement = require("revisa.placement")

-- comment is an open new-side comment on a.txt, with fields changed.
local function comment(fields)
    local found = {
        path = "a.txt",
        side = "new",
        status = "open",
        line_start = 2,
        line_end = 3,
        anchor_text = "local x = y",
        body = "Handle nil",
    }
    for key, value in pairs(fields) do
        found[key] = value
    end
    return found
end

-- withoutLines is a comment whose current lines are not known.
local function withoutLines(found)
    found.line_start, found.line_end = nil, nil
    return found
end

local newSide = { kind = "file", path = "a.txt", side = "new" }
local oldSide = {
    kind = "file", path = "a.txt", side = "old", commit = "base",
}
local changes = { kind = "changes", worktree = "/repo", branch = "feat/x" }

local cases = {
    {
        name = "new side, open",
        want = {
            kind = "anchored", sign = ">", group = "RevisaSign",
            line_start = 2, line_end = 3,
        },
    },
    {
        name = "new side, resolved, hidden",
        comment = comment({ status = "resolved", reply = "Fixed" }),
        want = { kind = "hidden" },
    },
    {
        name = "new side, resolved, shown",
        comment = comment({ status = "resolved", reply = "Fixed" }),
        show = true,
        want = {
            kind = "anchored", sign = "✓", group = "RevisaResolved",
        },
    },
    {
        name = "new side, outdated",
        comment = withoutLines(comment({ condition = "outdated" })),
        want = { kind = "marker", condition = "outdated" },
    },
    {
        name = "new side, unknown",
        comment = withoutLines(comment({ condition = "unknown" })),
        want = { kind = "marker", condition = "unknown" },
    },
    {
        name = "new side, outdated, stale range",
        comment = comment({ condition = "outdated" }),
        count = 2,
        want = { kind = "marker", condition = "outdated" },
    },
    {
        name = "new side, range past the end",
        comment = comment({ line_end = 9 }),
        want = { kind = "hidden", why = "no current lines" },
    },
    {
        name = "new side comment, old side target",
        target = oldSide,
        want = { kind = "hidden", why = "another side" },
    },
    {
        name = "old side, open, same commit",
        comment = comment({ side = "old", commit = "base" }),
        target = oldSide,
        want = {
            kind = "anchored", sign = ">", group = "RevisaSign",
            line_start = 2, line_end = 3,
        },
    },
    {
        name = "old side, open, other commit",
        comment = comment({ side = "old", commit = "head" }),
        target = oldSide,
        want = { kind = "hidden", why = "another commit" },
    },
    {
        name = "changes target",
        target = changes,
        want = { kind = "hidden", why = "not a file target" },
    },
    {
        name = "another file",
        comment = comment({ path = "b.txt" }),
        want = { kind = "hidden", why = "another file" },
    },
}

for _, case in ipairs(cases) do
    local got = placement.place(case.comment or comment({}),
        case.target or newSide, case.count or 5, case.show)
    for field, value in pairs(case.want) do
        H.equal(value, got[field])
    end
end

-- A marker carries the body and the anchor text it showed instead.
local marker = placement.place(
    withoutLines(comment({ condition = "outdated" })), newSide, 5, false)
H.equal({ { { "Handle nil", "RevisaComment" } } }, marker.body)
H.equal({ { { "was: local x = y", "RevisaWas" } } }, marker.was)
