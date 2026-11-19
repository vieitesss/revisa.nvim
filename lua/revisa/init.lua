local M = {}
local api = vim.api
local cli = require("revisa.cli")
local display = require("revisa.display")
local target = require("revisa.target")

local function refused(buf, what)
    if not vim.bo[buf].modified then
        return false
    end
    cli.error("save the buffer before " .. what)
    return true
end

local function addArgs(found, lines)
    local args = { "--path", found.path, "--lines", lines }
    if found.side == "old" then
        vim.list_extend(args, { "--side", "old",
            "--branch", found.branch, "--commit", found.commit })
    elseif found.branch ~= nil and found.branch ~= "" then
        vim.list_extend(args, { "--branch", found.branch })
    end
    return args
end

function M.refresh(buf)
    display.refresh(buf)
end

-- toggle_resolved shows or hides resolved comments in every loaded buffer
-- that has revisa marks.
function M.toggle_resolved()
    display.toggle_resolved()
end

function M.comment(first, last)
    local buf = api.nvim_get_current_buf()
    local found, err = target.file(target.of(buf))
    if not found then
        cli.error(err)
        return
    end
    if refused(buf, "commenting") then
        return
    end
    require("revisa.float").open(buf, last, function(body)
        if refused(buf, "commenting") then
            return false
        end
        local args = { "add" }
        vim.list_extend(args, addArgs(found, first .. "-" .. last))
        vim.list_extend(args, { "--body", body })
        local result = cli.run({ cwd = found.cwd }, args)
        if result == nil then
            return false
        end
        M.refresh(buf)
        return true
    end)
end

local function label(item)
    local where = item.comment.condition
    if item.first then
        where = "line " .. item.first
        if item.last ~= item.first then
            where = "lines " .. item.first .. "-" .. item.last
        end
    end
    return where .. ": " .. vim.split(item.comment.body, "\n")[1]
end

-- edit changes the body of a drawn comment: the one on the cursor line,
-- else one picked from the file's comments.
function M.edit()
    local buf = api.nvim_get_current_buf()
    display.refresh(buf)
    -- refresh has already said why a buffer has no file target.
    local found = target.file(target.of(buf))
    if not found then
        return
    end
    local line = api.nvim_win_get_cursor(0)[1]
    local items = display.at(buf, line)
    local function open(item)
        if not item then
            return
        end
        require("revisa.float").open(buf, item.last, function(body)
            if refused(buf, "editing") then
                return false
            end
            local result = cli.run({ cwd = found.cwd },
                { "edit", item.comment.id, "--body", body })
            if result == nil then
                return false
            end
            display.refresh(buf)
            return true
        end, item.comment.body)
    end
    if #items == 0 then
        cli.error("revisa: no comment to edit in this buffer")
    elseif #items == 1 then
        open(items[1])
    else
        vim.ui.select(items, {
            prompt = "Edit comment",
            format_item = label,
        }, open)
    end
end

function M.review(worktree)
    require("revisa.review").review(worktree)
end

function M.send(worktree)
    require("revisa.notify").send(worktree)
end

function M.setup(options)
    require("revisa.notify").setup(options or {})
end

return M
