local api = vim.api

local function open(path)
    vim.cmd.edit(vim.fn.fnameescape(path))
    return api.nvim_get_current_buf()
end

-- Where each drawn body starts, by the file line it sits under.
local function bodies(buf)
    local out = {}
    for _, mark in ipairs(H.marks(buf)) do
        local details = mark[4]
        if details.virt_lines then
            local text = {}
            for _, line in ipairs(details.virt_lines) do
                text[#text + 1] = line[1][1]
            end
            out[mark[2] + (details.virt_lines_above and 1 or 0)] = text
        end
    end
    return out
end

-- one open comment on lines 2-3; the resolved, old-side and other-file
-- comments stay hidden
local buf = open(vim.env.REVISA_FILE)
H.display(buf, 2, 3, { "Handle nil" })

require("revisa").toggle_resolved()
H.equal({
    { line = 2, text = ">", group = "RevisaSign" },
    { line = 3, text = ">", group = "RevisaSign" },
    { line = 4, text = "✓", group = "RevisaResolved" },
    { line = 5, text = "✓", group = "RevisaResolved" },
}, H.signs(buf))
H.equal({
    {
        row = 3,
        above = true,
        lines = { { { "Handle nil", "RevisaComment" } } },
    },
    {
        row = 5,
        above = true,
        lines = {
            { { "Resolved body", "RevisaComment" } },
            { { "reply: Fixed", "RevisaReply" } },
        },
    },
}, H.virtual(buf))
H.equal(nil, H.text(buf))

-- a comment whose lines the shortened buffer no longer has is hidden
api.nvim_buf_set_lines(buf, 3, -1, false, {})
H.equal(3, api.nvim_buf_line_count(buf))
require("revisa").refresh(buf)
H.display(buf, 2, 3, { "Handle nil" })

-- the old side of the base commit draws only its own comments
local lines = vim.split(vim.fn.system({ "git", "-C", vim.fn.getcwd(),
    "show", vim.env.REVISA_BASE .. ":src/a.lua" }), "\n", { plain = true })
if lines[#lines] == "" then
    table.remove(lines)
end
local old = api.nvim_create_buf(false, true)
vim.bo[old].swapfile = false
api.nvim_buf_set_name(old, "revisa://old/src/a.lua")
api.nvim_buf_set_lines(old, 0, -1, false, lines)
vim.b[old].revisa_target = {
    name = api.nvim_buf_get_name(old),
    worktree = vim.fn.getcwd(),
    path = "src/a.lua",
    side = "old",
    commit = vim.env.REVISA_BASE,
}
require("revisa.display").refresh(old)
H.equal({ { line = 2, text = ">", group = "RevisaSign" } }, H.signs(old))
H.equal({
    {
        row = 2,
        above = true,
        lines = { { { "Old side body", "RevisaComment" } } },
    },
}, H.virtual(old))
H.equal(nil, H.text(old))

-- a comment whose anchor text is gone draws in the end block
local gone = open(vim.env.REVISA_OUTDATED)
H.equal({}, H.signs(gone))
H.equal({
    {
        row = 2,
        above = true,
        lines = {
            { { "outdated (2)", "RevisaOutdated" } },
            { { "Gone line", "RevisaComment" } },
            { { "was: not here", "RevisaWas" } },
            { { "Another body", "RevisaComment" } },
            { { "was: also gone", "RevisaWas" } },
        },
    },
}, H.virtual(gone))
H.equal({ { "revisa: 2 outdated", "RevisaOutdated" } }, H.text(gone))

-- a non-file target draws nothing and says why
local messages = H.notifications()
local changes = api.nvim_create_buf(false, true)
vim.b[changes].revisa_session = {
    worktree = vim.fn.getcwd(), entries_by_line = {},
}
H.equal(nil, require("revisa.display").refresh(changes))
H.error(messages, "comments require a real file buffer")
H.equal({}, H.marks(changes))

-- bodies wrap to the cells a 20 column window leaves
local wrap = open(vim.env.REVISA_WRAP)
local win = api.nvim_open_win(wrap, true, {
    relative = "editor", row = 0, col = 0, width = 20, height = 9,
    style = "minimal",
})
vim.wo[win].signcolumn = "no"
require("revisa").refresh(wrap)
H.equal({
    [2] = { "before", "", "after" },
    [3] = { "short" },
    [4] = { "aaaa bbbb cccc dddd", "eeee" },
    [5] = { "    local value =", "f(a, b)" },
    [6] = { "abcdefghijklmnopqrst", "uvwxyz" },
    [7] = { "one  two   three", "four five" },
    [8] = { "ñandú ñandú ñandú", "ñandú" },
    [10] = { "first second third", "fourth fifth", "reply: done here",
        "with more details" },
}, bodies(wrap))

-- the was: lines of an end block comment wrap too
local long = open(vim.env.REVISA_OUTDATED_LONG)
local also = api.nvim_open_win(long, true, {
    relative = "editor", row = 0, col = 0, width = 20, height = 6,
    style = "minimal",
})
vim.wo[also].signcolumn = "no"
require("revisa").refresh(long)
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
}, H.virtual(long))
api.nvim_win_close(also, true)
api.nvim_win_close(win, true)
