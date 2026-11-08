-- The plugin composes a buffer's review target locally; it does not ask the
-- CLI for one. A `revisa context` command was rejected: it would add a process
-- to every refresh and a second JSON contract, while the two discoveries must
-- agree only on the path used to store comments, which the process tests pin.
-- If they drift, the headless target test fails before the buffer does.

local M = {}
local uv = vim.uv
local buffer = require("revisa.buffer")

-- require_file is the one reason every buffer kind that shows no file
-- reports, so a reviewer learns the same rule everywhere.
local require_file = "revisa: comments require a real file buffer"

-- real resolves every symlink in path, so two spellings of one worktree
-- compare equal.
local function real(path)
    if type(path) ~= "string" or path == "" then
        return path
    end
    return uv.fs_realpath(path) or path
end

-- fromSession is the target of a changes buffer, built from the session
-- the buffer stores.
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
        entries = session.entries,
    }
end

-- fromPin is the target diff.open pinned on one of its buffers.
local function fromPin(pin)
    local worktree = real(pin.worktree)
    return {
        kind = "file",
        worktree = worktree,
        cwd = worktree,
        path = pin.path,
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
    if type(pin) == "table" then
        return fromPin(pin)
    end
    local context, why = buffer.context(buf)
    if not context then
        return nil, why
    end
    local worktree = real(context.cwd)
    return {
        kind = "file",
        worktree = worktree,
        cwd = worktree,
        path = context.path,
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
