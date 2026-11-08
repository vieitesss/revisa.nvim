local M = {}
local api = vim.api
local cli = require("revisa.cli")
local display = require("revisa.display")
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

function M.review(worktree)
    require("revisa.review").review(worktree)
end

function M.beside(worktree)
    require("revisa.rex").beside(worktree)
end

function M.send(worktree)
    require("revisa.notify").send(worktree)
end

function M.setup(options)
    require("revisa.notify").setup(options or {})
end

return M
