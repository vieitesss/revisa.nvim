local api = vim.api
local target = require("revisa.target")

-- entryAt returns the change the changes buffer shows on the line text.
local function entryAt(buf, text)
    for line, current in ipairs(api.nvim_buf_get_lines(buf, 0, -1, false)) do
        if current == text then
            return vim.b[buf].revisa_session.entries[line]
        end
    end
    error("no line " .. text)
end

-- sides returns the target of the old and new window of the open diff tab.
local function sides()
    local old, new
    for _, win in ipairs(api.nvim_tabpage_list_wins(0)) do
        local found = target.of(api.nvim_win_get_buf(win))
        if found.side == "old" then
            old = found
        else
            new = found
        end
    end
    assert(old and new, "diff sides not found")
    return old, new
end

-- review opens the changes list of the reviewed worktree and returns it.
local function review()
    require("revisa").review(vim.env.REVISA_TARGET)
    local buf = api.nvim_get_current_buf()
    assert(target.of(buf).kind == "changes", "not the changes buffer")
    return buf
end

-- diff opens the diff of the change on the changes line text and returns
-- the targets of both its sides.
local function diff(list, text)
    local change = entryAt(list, text)
    assert(change, text .. " is not in the changes list")
    local session = vim.b[list].revisa_session
    require("revisa.diff").open({
        worktree = session.worktree,
        branch = session.branch,
        commit = session.base,
        status = change.status,
        path = change.path,
        old_path = change.old_path,
    })
    return sides()
end

-- check_file asserts the target of a real file buffer in the worktree.
local function check_file(buf, worktree)
    H.equal({
        kind = "file",
        worktree = worktree,
        cwd = worktree,
        path = "src/a.lua",
        side = "new",
    }, target.of(buf))
end

-- check_scratch asserts the reason a buffer that shows no file reports.
local function check_scratch(name, why)
    local buf = api.nvim_create_buf(false, true)
    vim.bo[buf].buftype = ""
    api.nvim_buf_set_name(buf, name)
    local found, reason = target.of(buf)
    H.equal(nil, found)
    H.equal(why, reason)
end

local main = vim.env.REVISA_MAIN
local linked = vim.env.REVISA_LINKED

-- A real file buffer resolves to the file version the worktree holds.
H.open()
check_file(api.nvim_get_current_buf(), linked)
vim.cmd.enew()
vim.cmd.edit(vim.fn.fnameescape(vim.env.REVISA_MAIN_FILE))
check_file(api.nvim_get_current_buf(), main)

-- A changes buffer resolves to the whole list, with its head and entries.
local list = review()
local changes = target.of(list)
local entries = changes.entries
changes.entries = nil
H.equal({
    kind = "changes",
    worktree = linked,
    cwd = linked,
    branch = "feat/x",
    head = vim.env.REVISA_HEAD,
    base = vim.env.REVISA_BASE,
    default_branch = "main",
}, changes)
-- Entries stay keyed by the buffer line that shows them.
H.equal({ status = "?", path = "notes.txt" },
    entryAt(list, "? notes.txt"))
H.equal({ status = "D", path = "src/gone.go" },
    entryAt(list, "D src/gone.go"))
H.equal({ status = "R", path = "src/renamed.go",
    old_path = "src/moved.go" }, entryAt(list, "R src/moved.go -> " ..
    "src/renamed.go"))
-- A header line shows no change.
H.equal(nil, require("revisa.review").entry(list, 2))

-- The changes buffer is no file version: commenting there is refused.
local found, why = target.file(target.of(list))
H.equal(nil, found)
H.equal("revisa: comments require a real file buffer", why)

-- Both sides of a diff carry the reviewed branch, so the old side knows
-- the branch its comment belongs to.
local old, new = diff(list, "M src/a.lua")
H.equal({
    kind = "file",
    worktree = linked,
    cwd = linked,
    path = "src/a.lua",
    side = "old",
    commit = vim.env.REVISA_BASE,
    branch = "feat/x",
}, old)
H.equal({
    kind = "file",
    worktree = linked,
    cwd = linked,
    path = "src/a.lua",
    side = "new",
    branch = "feat/x",
}, new)
require("revisa.diff").close()

-- A deleted file exists only on the old side, but its new side is pinned
-- too, so the buffer names the reviewed branch.
local deletedOld, deletedNew = diff(list, "D src/gone.go")
H.equal("old", deletedOld.side)
H.equal("src/gone.go", deletedOld.path)
H.equal({
    kind = "file",
    worktree = linked,
    cwd = linked,
    path = "src/gone.go",
    side = "new",
    branch = "feat/x",
}, deletedNew)
require("revisa.diff").close()

-- A buffer that shows no file reports why instead of guessing one.
check_scratch(vim.env.REVISA_FILE .. ".missing",
    "revisa: comments require a file on disk")
check_scratch("tool://old-side/src/a.lua",
    "revisa: cannot comment in a tool:// buffer; open the file or use " ..
    ":Revisa review")
