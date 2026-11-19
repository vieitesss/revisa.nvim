local api = vim.api
local display = require("revisa.display")
local buf = api.nvim_create_buf(false, true)

local function comment(fields)
    return vim.tbl_extend("force", {
        path = "a.txt",
        side = "new",
        status = "open",
        line_start = 2,
        line_end = 3,
        anchor_text = "local x = y",
        body = "Handle nil",
    }, fields or {})
end

local newSide = { kind = "file", path = "a.txt", side = "new" }
local oldSide = {
    kind = "file", path = "a.txt", side = "old", commit = "base",
}

local cases = {
    { want = "open" },
    { comment = { status = "resolved", reply = "Fixed" }, want = "hidden" },
    {
        comment = { status = "resolved", reply = "Fixed" },
        show = true,
        want = "resolved",
    },
    { comment = { condition = "outdated" }, count = 2, want = "outdated" },
    { comment = { line_end = 9 }, want = "hidden" },
    { target = oldSide, want = "hidden" },
    {
        comment = { side = "old", commit = "base" },
        target = oldSide,
        want = "open",
    },
    {
        comment = { side = "old", commit = "head" },
        target = oldSide,
        want = "hidden",
    },
    { target = { kind = "changes" }, want = "hidden" },
    { comment = { path = "b.txt" }, want = "hidden" },
}

for _, case in ipairs(cases) do
    api.nvim_buf_set_lines(buf, 0, -1, false,
        case.count == 2 and { "one", "two" }
            or { "one", "two", "three", "four", "five" })
    display.draw(buf, case.target or newSide, { comment(case.comment) },
        case.show)
    if case.want == "open" then
        H.display(buf, 2, 3, { "Handle nil" })
    elseif case.want == "resolved" then
        H.resolved(buf, 2, 3, {
            { "Handle nil", "RevisaComment" },
            { "reply: Fixed", "RevisaReply" },
        })
    elseif case.want == "outdated" then
        H.equal({}, H.signs(buf))
        H.equal({
            {
                row = 2,
                above = true,
                lines = {
                    { { "outdated (1)", "RevisaOutdated" } },
                    { { "Handle nil", "RevisaComment" } },
                    { { "was: local x = y", "RevisaWas" } },
                },
            },
        }, H.virtual(buf))
    else
        H.equal({}, H.signs(buf))
        H.equal({}, H.virtual(buf))
        H.equal(nil, H.text(buf))
    end
end

api.nvim_buf_set_lines(buf, 0, -1, false, { "one", "two", "three" })
local win = api.nvim_open_win(buf, true, {
    relative = "editor", row = 0, col = 0, width = 20, height = 6,
    style = "minimal",
})
vim.wo[win].signcolumn = "no"
for _, case in ipairs({
    { "before\n\nafter", { "before", "", "after" } },
    { "short", { "short" } },
    { "aaaa bbbb cccc dddd eeee", { "aaaa bbbb cccc dddd", "eeee" } },
    { "    local value = f(a, b)", { "    local value =", "f(a, b)" } },
    { "abcdefghijklmnopqrstuvwxyz", { "abcdefghijklmnopqrst", "uvwxyz" } },
    { "one  two   three  four five", { "one  two   three", "four five" } },
    { "ñandú ñandú ñandú ñandú", { "ñandú ñandú ñandú", "ñandú" } },
}) do
    display.draw(buf, newSide, { comment({ body = case[1] }) }, false)
    H.display(buf, 2, 3, case[2])
end

display.draw(buf, newSide, { comment({
    status = "resolved",
    body = "first second third fourth fifth",
    reply = "done here with more details",
}) }, true)
H.resolved(buf, 2, 3, {
    { "first second third", "RevisaComment" },
    { "fourth fifth", "RevisaComment" },
    { "reply: done here", "RevisaReply" },
    { "with more details", "RevisaReply" },
})
display.draw(buf, newSide, { comment({
    condition = "outdated",
    body = "Gone line",
    anchor_text = "local value = something",
}) }, false)
H.equal({
    {
        row = 3,
        above = true,
        lines = {
            { { "outdated (1)", "RevisaOutdated" } },
            { { "Gone line", "RevisaComment" } },
            { { "was: local value =", "RevisaWas" } },
            { { "something", "RevisaWas" } },
        },
    },
}, H.virtual(buf))
api.nvim_win_close(win, true)
