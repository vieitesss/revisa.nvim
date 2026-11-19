local M = {}
local uv = vim.uv

function M.file(buf)
    local name = vim.api.nvim_buf_get_name(buf)
    local scheme = name:match("^(%w[%w+.-]*)://")
    if scheme then
        return nil, "revisa: cannot comment in a " .. scheme .. ":// " ..
            "buffer; open the file or use :Revisa review"
    end
    if vim.bo[buf].buftype ~= "" then
        return nil, "revisa: comments require a real file buffer"
    end
    local info = uv.fs_stat(name)
    if not info or info.type ~= "file" then
        return nil, "revisa: comments require a file on disk"
    end
    return vim.fs.normalize(name)
end

local function contains(root, path)
    return path:sub(1, #root + 1) == root .. "/"
        or root == "/"
end

-- M.context returns where the CLI runs for buf's file, and the file's
-- absolute path. The CLI maps that path to the project and the store.
function M.context(buf)
    local path, err = M.file(buf)
    if not path then
        return nil, err
    end
    local cwd = vim.fn.getcwd()
    if not contains(cwd, path) then
        cwd = vim.fs.dirname(path)
    end
    return { cwd = cwd, path = path }
end

return M
