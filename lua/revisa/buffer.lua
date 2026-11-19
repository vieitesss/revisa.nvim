local M = {}
local uv = vim.uv

-- Neovim resolves directory links in the full name, but keeps the lexical
-- name in bufname() until a directory change. Save it while it is available.
function M.remember(buf, replace)
    local name = vim.api.nvim_buf_get_name(buf)
    local saved = vim.b[buf].revisa_file
    if not replace and type(saved) == "table" and saved.name == name then
        return
    end
    local path = vim.fn.bufname(buf)
    if path ~= "" and path:sub(1, 1) ~= "/" then
        path = vim.fn.getcwd() .. "/" .. path
    end
    vim.b[buf].revisa_file = {
        name = name,
        path = vim.fs.normalize(path),
    }
end

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
    local saved = vim.b[buf].revisa_file
    local path = name
    if type(saved) == "table" and saved.name == name then
        path = saved.path
    end
    local info = uv.fs_stat(path)
    if not info or info.type ~= "file" then
        return nil, "revisa: comments require a file on disk"
    end
    return vim.fs.normalize(path)
end

local function contains(root, path)
    if path:sub(1, #root + 1) == root .. "/" or root == "/" then
        return true
    end
    -- An absolute name may spell the working directory through an alias.
    -- Compare directories without rewriting the file's lexical suffix.
    for dir in vim.fs.parents(path) do
        if uv.fs_realpath(dir) == root then
            return true
        end
    end
    return false
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
