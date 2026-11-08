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

function M.context(buf)
    local path, err = M.file(buf)
    if not path then
        return nil, err
    end
    local dir = vim.fs.dirname(path)
    local git = { code = 1 }
    if vim.fn.executable("git") == 1 then
        git = vim.system({
            "git", "-C", dir, "rev-parse", "--show-toplevel",
        }, { text = true }):wait()
    end
    local cwd, root
    if git.code == 0 then
        root = git.stdout:gsub("\n$", "")
        cwd = root
    else
        cwd = vim.fn.getcwd()
        if not contains(cwd, path) then
            cwd = dir
        end
        root = cwd
        local ancestor = cwd
        while true do
            local info = uv.fs_stat(ancestor .. "/.revisa")
            if info and info.type == "directory" then
                root = ancestor
                break
            end
            local parent = vim.fs.dirname(ancestor)
            if parent == ancestor then
                break
            end
            ancestor = parent
        end
    end
    return {
        cwd = cwd,
        path = path:sub(#root + (root == "/" and 1 or 2)),
    }
end

return M
