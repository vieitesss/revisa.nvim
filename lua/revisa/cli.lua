local M = {}

local source = debug.getinfo(1, "S").source:sub(2)
local fallback = vim.fn.fnamemodify(source, ":h:h:h") .. "/bin/revisa"

function M.error(message)
    vim.notify(message, vim.log.levels.ERROR)
end

function M.run(context, args)
    local binary = vim.fn.exepath("revisa")
    if binary == "" then
        if vim.fn.executable(fallback) == 0 then
            M.error("revisa not found: tried revisa on PATH and " .. fallback)
            return nil
        end
        binary = fallback
    end
    local command = { binary }
    vim.list_extend(command, args)
    local ok, result = pcall(function()
        return vim.system(command, {
            cwd = context.cwd,
            text = true,
        }):wait()
    end)
    if not ok then
        M.error(tostring(result))
        return nil
    end
    if result.code ~= 0 then
        local message = vim.trim(result.stderr or "")
        if message == "" then
            message = "revisa failed (exit " .. result.code .. ")"
        end
        M.error(message)
        return nil
    end
    return result.stdout
end

-- json runs one CLI command in context and returns its decoded output, or
-- nil. A bad payload is reported here, so every caller reports it once
-- and the same way.
function M.json(context, args)
    local output = M.run(context, args)
    if not output then
        return nil
    end
    local ok, data = pcall(vim.json.decode, output)
    if not ok then
        M.error("revisa: invalid JSON from CLI: " .. tostring(data))
        return nil
    end
    return data
end

return M
