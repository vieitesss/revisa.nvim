local M = {}
local api = vim.api
local cli = require("revisa.cli")
local diff = require("revisa.diff")

local function decode(context, args)
    local output = cli.run(context, args)
    if not output then
        return nil
    end
    local ok, data = pcall(vim.json.decode, output)
    if not ok then
        cli.error("revisa: invalid JSON from CLI: " .. tostring(data))
        return nil
    end
    return data
end

local function label(branch)
    if branch == nil or branch == "" then
        return "(detached)"
    end
    return branch
end

-- fetchChanges reads one worktree's session changes through the CLI.
-- request is a worktree path or branch, empty for the worktree of the
-- current directory.
local function fetchChanges(request)
    local args = { "changes", "--json" }
    if request and request ~= "" then
        vim.list_extend(args, { "--worktree", request })
    end
    return decode({ cwd = vim.fn.getcwd() }, args)
end

local function name(data)
    local branch = data.branch
    if branch == nil or branch == "" then
        branch = "detached"
    end
    return "revisa://changes/" .. branch
end

-- build returns the buffer lines and the entry of each of those lines.
local function build(data)
    local lines = {
        "worktree " .. data.worktree,
        "branch " .. label(data.branch),
        "base " .. data.base:sub(1, 7) ..
            " (" .. data.default_branch .. ")",
        "",
    }
    local entries = {}
    for _, entry in ipairs(data.entries) do
        local text = entry.status .. " " .. entry.path
        if entry.status == "R" then
            text = "R " .. entry.old_path .. " -> " .. entry.path
        end
        lines[#lines + 1] = text
        entries[#lines] = entry
    end
    return lines, entries
end

-- draw replaces the buffer contents, keeping the cursor on its line.
local function draw(buf, data)
    local lines, entries = build(data)
    local cursor = api.nvim_win_get_cursor(0)[1]
    vim.bo[buf].modifiable = true
    api.nvim_buf_set_lines(buf, 0, -1, false, lines)
    vim.bo[buf].modifiable = false
    vim.b[buf].revisa_changes = entries
    vim.b[buf].revisa_worktree = data.worktree
    vim.b[buf].revisa_base = data.base
    api.nvim_win_set_cursor(0, { math.min(cursor, #lines), 0 })
end

-- entry returns the change on line of buf, or nil on a header line.
-- The diff view reads the entry under the cursor through this.
function M.entry(buf, line)
    local entries = vim.b[buf].revisa_changes
    if type(entries) ~= "table" then
        return nil
    end
    local change = entries[line or api.nvim_win_get_cursor(0)[1]]
    if type(change) ~= "table" then
        return nil
    end
    return change
end

local function openEntry(buf)
    local change = M.entry(buf)
    if not change then
        return
    end
    local worktree = vim.b[buf].revisa_worktree
    if change.status == "D" then
        cli.error("revisa: " .. change.path .. " is deleted in " .. worktree)
        return
    end
    local previous = 0
    pcall(function()
        previous = vim.fn.win_getid(vim.fn.winnr("#"))
    end)
    if previous ~= 0 and previous ~= api.nvim_get_current_win() then
        api.nvim_set_current_win(previous)
    end
    vim.cmd.edit(vim.fn.fnameescape(worktree .. "/" .. change.path))
end

local function openDiff(buf)
    local change = M.entry(buf)
    if not change then
        return
    end
    diff.open({
        worktree = vim.b[buf].revisa_worktree,
        commit = vim.b[buf].revisa_base,
        status = change.status,
        path = change.path,
        old_path = change.old_path,
    })
end

local function reload(buf)
    local data = fetchChanges(vim.b[buf].revisa_worktree)
    if data then
        draw(buf, data)
    end
end

local function close()
    if #api.nvim_list_wins() > 1 then
        vim.cmd.close()
        return
    end
    vim.cmd.enew()
end

local function isNamed(name)
    for _, buf in ipairs(api.nvim_list_bufs()) do
        if api.nvim_buf_get_name(buf) == name then
            return buf
        end
    end
    return 0
end

function M.open(request)
    local data = fetchChanges(request)
    if not data then
        return
    end
    local existing = isNamed(name(data))
    if existing ~= 0 then
        api.nvim_buf_delete(existing, { force = true })
    end
    local buf = api.nvim_create_buf(false, true)
    vim.bo[buf].bufhidden = "wipe"
    api.nvim_buf_set_name(buf, name(data))
    vim.cmd.split()
    api.nvim_win_set_buf(0, buf)
    draw(buf, data)
    vim.keymap.set("n", "<CR>", function()
        openDiff(buf)
    end, { buffer = buf, nowait = true, desc = "revisa: open the diff" })
    vim.keymap.set("n", "o", function()
        openEntry(buf)
    end, { buffer = buf, nowait = true, desc = "revisa: open the file" })
    vim.keymap.set("n", "R", function()
        reload(buf)
    end, { buffer = buf, desc = "revisa: reload the changes" })
    vim.keymap.set("n", "q", close,
        { buffer = buf, desc = "revisa: close the changes" })
end

function M.pick()
    local worktrees = decode({ cwd = vim.fn.getcwd() },
        { "worktrees", "--json" })
    if not worktrees then
        return
    end
    local labels = {}
    for i, worktree in ipairs(worktrees) do
        labels[i] = label(worktree.branch) .. "  " .. worktree.path
    end
    vim.ui.select(labels, { prompt = "revisa: review a worktree" },
        function(_, index)
            if not index then
                return
            end
            M.open(worktrees[index].path)
        end)
end

function M.review(worktree)
    if worktree and worktree ~= "" then
        M.open(worktree)
        return
    end
    M.pick()
end

return M
