local M = {}
local api = vim.api
local cli = require("revisa.cli")

local binaryProbeBytes = 8000

local function readProbe(path)
    local file = io.open(path, "rb")
    if not file then
        return nil
    end
    local bytes = file:read(binaryProbeBytes)
    file:close()
    return bytes
end

local function isBinary(bytes)
    return bytes ~= nil and bytes:find("\0", 1, true) ~= nil
end

local function name(commit, path)
    return "revisa://" .. commit:sub(1, 7) .. "/" .. path
end

local function show(worktree, commit, path)
    local command = { "git", "-C", worktree, "show", commit .. ":" .. path }
    local output, why = cli.exec(command, {}, "git show")
    if not output then
        cli.error(why)
    end
    return output
end

local function lines(text)
    local list = vim.split(text, "\n", { plain = true })
    if list[#list] == "" then
        table.remove(list)
    end
    return list
end

-- wipe deletes a buffer that already has this name, because buffer
-- names are unique.
local function wipe(named)
    for _, buf in ipairs(api.nvim_list_bufs()) do
        if api.nvim_buf_get_name(buf) == named then
            api.nvim_buf_delete(buf, { force = true })
        end
    end
end

local function scratch(named, text, path)
    wipe(named)
    local buf = api.nvim_create_buf(false, true)
    vim.bo[buf].bufhidden = "wipe"
    vim.bo[buf].swapfile = false
    api.nvim_buf_set_name(buf, named)
    if path then
        vim.bo[buf].filetype =
            vim.filetype.match({ filename = path }) or ""
    end
    api.nvim_buf_set_lines(buf, 0, -1, false, lines(text))
    vim.bo[buf].modifiable = false
    return buf
end

function M.close()
    if #api.nvim_list_tabpages() > 1 then
        vim.cmd.tabclose()
    end
end

local function pin(buf, worktree, branch, path, side, commit)
    vim.b[buf].revisa_target = {
        name = api.nvim_buf_get_name(buf),
        worktree = worktree,
        branch = branch,
        path = path,
        side = side,
        commit = commit,
    }
end

-- open shows one change side by side in a new tab: the content at the
-- base on the left, the worktree file on the right. Neovim diff mode
-- does the rest. change has worktree, branch, commit, status, path and
-- old_path.
function M.open(change)
    local oldPath = change.old_path or change.path
    local old = ""
    if change.status ~= "A" and change.status ~= "?" then
        old = show(change.worktree, change.commit, oldPath)
        if old == nil then
            return
        end
    end
    local file = change.worktree .. "/" .. change.path
    local new
    if change.status ~= "D" then
        new = readProbe(file)
    end
    if isBinary(old) or isBinary(new) then
        cli.error("revisa: " .. change.path .. " is binary")
        return
    end
    vim.cmd.tabnew()
    local newWin
    if change.status == "D" then
        local buf = scratch("revisa://deleted/" .. change.path, "", nil)
        api.nvim_win_set_buf(0, buf)
        pin(buf, change.worktree, change.branch, change.path, "new")
        -- This buffer is never read, so its end block of outdated
        -- comments is drawn here instead of on BufReadPost.
        require("revisa.display").refresh(buf)
        newWin = api.nvim_get_current_win()
    else
        vim.cmd.edit(vim.fn.fnameescape(file))
        pin(api.nvim_get_current_buf(), change.worktree, change.branch,
            change.path, "new")
        newWin = api.nvim_get_current_win()
    end
    vim.cmd.vsplit()
    local oldWin = api.nvim_get_current_win()
    local buf = scratch(name(change.commit, oldPath), old, oldPath)
    api.nvim_win_set_buf(oldWin, buf)
    pin(buf, change.worktree, change.branch, oldPath, "old", change.commit)
    -- The old side is never read from disk, so no BufReadPost fires and
    -- its comments are drawn here instead.
    require("revisa.display").refresh(buf)
    vim.keymap.set("n", "q", M.close,
        { buffer = buf, desc = "revisa: close the diff" })
    vim.cmd.diffthis()
    api.nvim_set_current_win(newWin)
    vim.cmd.diffthis()
    api.nvim_set_current_win(oldWin)
end

return M
