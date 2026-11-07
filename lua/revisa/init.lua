local M = {}
local api = vim.api
local buffer = require("revisa.buffer")
local cli = require("revisa.cli")
local display = require("revisa.display")

local show_resolved = false

function M.refresh(buf)
    buf = buf or api.nvim_get_current_buf()
    local context, err = buffer.context(buf)
    if not context then
        cli.error(err)
        return
    end
    local output = cli.run(context, { "list", "--json" })
    if not output then
        return
    end
    local ok, comments = pcall(vim.json.decode, output)
    if not ok then
        cli.error("revisa: invalid JSON from CLI: " .. tostring(comments))
        return
    end
    display.draw(buf, context.path, comments, show_resolved)
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
    local context, err = buffer.context(buf)
    if not context then
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
        local result = cli.run(context, {
            "add", "--path", context.path,
            "--lines", first .. "-" .. last, "--body", body,
        })
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

function M.send(worktree)
    require("revisa.send").send(worktree)
end

function M.setup(options)
    require("revisa.send").setup(options or {})
end

return M
