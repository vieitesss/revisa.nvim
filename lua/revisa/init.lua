local M = {}
local api = vim.api
local cli = require("revisa.cli")
local display = require("revisa.display")
local placement = require("revisa.placement")
local target = require("revisa.target")

local show_resolved = false

-- addArgs are the flags that name the file version a comment is made on.
-- The old side always names its branch and commit; a new side names its
-- branch when the target has one, and lets the CLI infer it otherwise.
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
    buf = buf or api.nvim_get_current_buf()
    local found, err = target.file(target.of(buf))
    if not found then
        cli.error(err)
        return
    end
    local comments = cli.json({ cwd = found.cwd }, { "list", "--json" })
    if not comments then
        return
    end
    display.draw(buf, found, comments, show_resolved)
end

-- toggle_resolved shows or hides resolved comments in every loaded buffer
-- that has revisa marks.
function M.toggle_resolved()
    show_resolved = not show_resolved
    for _, buf in ipairs(display.buffers()) do
        M.refresh(buf)
    end
end

function M.comment(first, last)
    local buf = api.nvim_get_current_buf()
    local found, err = target.file(target.of(buf))
    if not found then
        cli.error(err)
        return
    end
    if vim.bo[buf].modified then
        cli.error("save the buffer before commenting")
        return
    end
    require("revisa.float").open(buf, last, function(body)
        if vim.bo[buf].modified then
            cli.error("save the buffer before commenting")
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

-- editable lists the comments of buf that are drawn, with the line the
-- editor opens below: the last line of an anchored one, the last line of
-- the file for one in the end block. The ones on line come first.
local function editable(buf, found, comments, line)
    local count = api.nvim_buf_line_count(buf)
    local here, all = {}, {}
    for _, comment in ipairs(comments) do
        local placed = placement.place(comment, found, count, show_resolved,
            80)
        if placed.kind == "anchored" then
            local item = { comment = comment, last = placed.line_end }
            all[#all + 1] = item
            if placed.line_start <= line and line <= placed.line_end then
                here[#here + 1] = item
            end
        elseif placed.kind == "marker" then
            all[#all + 1] = { comment = comment, last = count }
        end
    end
    return #here > 0 and here or all
end

-- label names one comment in the picker: where it is, then its first line.
local function label(item)
    local comment = item.comment
    local where = comment.condition
    if type(comment.line_start) == "number" then
        where = "line " .. comment.line_start
        if comment.line_end ~= comment.line_start then
            where = "lines " .. comment.line_start .. "-" .. comment.line_end
        end
    end
    return where .. ": " .. vim.split(comment.body, "\n")[1]
end

-- edit changes the body of a drawn comment: the one on the cursor line,
-- else one picked from the file's comments.
function M.edit()
    local buf = api.nvim_get_current_buf()
    local found, err = target.file(target.of(buf))
    if not found then
        cli.error(err)
        return
    end
    local comments = cli.json({ cwd = found.cwd }, { "list", "--json" })
    if not comments then
        return
    end
    local line = api.nvim_win_get_cursor(0)[1]
    local items = editable(buf, found, comments, line)
    local function open(item)
        if not item then
            return
        end
        require("revisa.float").open(buf, item.last, function(body)
            local result = cli.run({ cwd = found.cwd },
                { "edit", item.comment.id, "--body", body })
            if result == nil then
                return false
            end
            M.refresh(buf)
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
