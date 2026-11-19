-- The plugin hands absolute paths to the CLI, which alone maps a file to
-- its project and its store. See docs/adr/0003-cli-maps-files.md.

local M = {}
local uv = vim.uv
local buffer = require("revisa.buffer")

local require_file = "revisa: comments require a real file buffer"

local function real(path)
    if type(path) ~= "string" or path == "" then
        return path
    end
    return uv.fs_realpath(path) or path
end

local function fromSession(session)
    local worktree = real(session.worktree)
    return {
        kind = "changes",
        worktree = worktree,
        cwd = worktree,
        branch = session.branch,
        head = session.head,
        base = session.base,
        default_branch = session.default_branch,
        entries_by_line = session.entries_by_line,
    }
end

local function fromPin(pin)
    local worktree = real(pin.worktree)
    return {
        kind = "file",
        worktree = worktree,
        cwd = worktree,
        absolute = worktree .. "/" .. pin.path,
        side = pin.side,
        commit = pin.commit,
        branch = pin.branch,
    }
end

-- M.of returns what buf shows: a changes list, or one file version of the
-- project. why says why it shows neither. It is computed per call and
-- never cached.
function M.of(buf)
    local session = vim.b[buf].revisa_session
    if type(session) == "table" then
        return fromSession(session)
    end
    local pin = vim.b[buf].revisa_target
    if type(pin) == "table"
        and pin.name == vim.api.nvim_buf_get_name(buf) then
        return fromPin(pin)
    end
    local context, why = buffer.context(buf)
    if not context then
        return nil, why
    end
    return {
        kind = "file",
        cwd = real(context.cwd),
        absolute = context.path,
        side = "new",
    }
end

-- M.file returns a target that names one file version, which commenting
-- and drawing comments need, or nil and the reason. The reason target.of
-- gave wins, so a buffer of another tool hears which buffer it is.
function M.file(target, why)
    if not target then
        return nil, why or require_file
    end
    if target.kind ~= "file" then
        return nil, require_file
    end
    return target
end

return M
