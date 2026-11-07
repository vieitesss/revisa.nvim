local M = {}
local api = vim.api
local buffer = require("revisa.buffer")
local cli = require("revisa.cli")

function M.refresh(buf)
    buf = buf or api.nvim_get_current_buf()
    local context, err = buffer.context(buf)
    if not context then
        cli.error(err)
        return
    end
    local output = cli.run(context, { "list", "--status", "open", "--json" })
    if not output then
        return
    end
    local ok, comments = pcall(vim.json.decode, output)
    if not ok then
        cli.error("revisa: invalid JSON from CLI: " .. tostring(comments))
        return
    end
    require("revisa.display").draw(buf, context.path, comments)
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

function M.setup()
end

return M
