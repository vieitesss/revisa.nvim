local M = {}

local source = debug.getinfo(1, "S").source:sub(2)
local fallback = vim.fn.fnamemodify(source, ":h:h:h") .. "/bin/revisa"

function M.error(message)
    vim.notify(message, vim.log.levels.ERROR)
end

-- exec runs command to its end and returns its stdout, or nil and why it
-- failed: the error starting it, its stderr, else "<what> failed (exit
-- N)". It reports nothing itself, so each caller decides how to show it.
function M.exec(command, options, what)
    local ok, result = pcall(function()
        return vim.system(command, vim.tbl_extend("force",
            { text = true }, options)):wait()
    end)
    if not ok then
        return nil, tostring(result)
    end
    if result.code ~= 0 then
        local message = vim.trim(result.stderr or "")
        if message == "" then
            message = what .. " failed (exit " .. result.code .. ")"
        end
        return nil, message
    end
    return result.stdout or ""
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
    local output, why = M.exec(command, { cwd = context.cwd }, "revisa")
    if not output then
        M.error(why)
    end
    return output
end

-- decode parses the JSON output of from, returning the data, or nil and
-- why it is not JSON.
function M.decode(output, from)
    local ok, data = pcall(vim.json.decode, output)
    if not ok then
        return nil, "revisa: invalid JSON from " .. from .. ": "
            .. tostring(data)
    end
    return data
end

-- json runs one CLI command in context and returns its decoded output, or
-- nil. A bad payload is reported here, so every caller reports it once
-- and the same way.
function M.json(context, args)
    local output = M.run(context, args)
    if not output then
        return nil
    end
    local data, why = M.decode(output, "CLI")
    if why then
        M.error(why)
    end
    return data
end

return M
