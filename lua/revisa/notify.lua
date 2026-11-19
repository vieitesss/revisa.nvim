local M = {}
local api = vim.api
local cli = require("revisa.cli")
local review = require("revisa.review")
local target = require("revisa.target")

-- A notifier is one revisa.notify.<name> module:
--
--   available() -> ok, why
--   recipients(worktree) -> list of { harness, cwd, state, location? }
--     plus what deliver needs, or nil after reporting why it failed; a
--     notifier that does not filter by worktree ignores the argument
--   deliver(recipient, message, branch) -> nil, error text or "retry"
--   confirm = true asks before typing into the recipient
--
-- A notifier never builds a label: this module finds each recipient's
-- branch, aligns the columns, orders the list and names the recipient it
-- sent to.

local settings = { notify = "auto" }
local allowed = { auto = true, none = true, recado = true, rex = true }

local pickPrompt = "revisa: send the comments to"

local offHint = 'revisa: :Revisa send is off, setup({ notify = "auto" |'
    .. ' "recado" | "rex" })'

local noNotifier = "revisa: :Revisa send needs rex, with REX_SESSION set"
    .. " and rex on PATH, or recado on PATH"

function M.setup(options)
    local notify = options.notify
    if notify == nil then
        return
    end
    if not allowed[notify] then
        cli.error("revisa: notify must be auto, none, recado or rex, got "
            .. tostring(notify))
        return
    end
    settings.notify = notify
end

local function shellQuote(text)
    return "'" .. text:gsub("'", "'\\''") .. "'"
end

local function message(branch)
    local quoted = shellQuote(branch)
    return "revisa: review comments are waiting on branch " .. quoted
        .. ". Run: revisa list --branch " .. quoted .. " --status open."
        .. " Answer each with revisa resolve, not recado reply."
end

-- git asks git in dir one question, nil when git cannot answer.
local function git(dir, args)
    local command = { "git", "-C", dir }
    vim.list_extend(command, args)
    local ok, result = pcall(function()
        return vim.system(command, { text = true }):wait()
    end)
    if not ok or result.code ~= 0 then
        return nil
    end
    local answer = vim.trim(result.stdout or "")
    if answer == "" then
        return nil
    end
    return answer
end

-- root is the worktree that holds dir, nil outside git.
local function root(dir)
    local top = git(dir, { "rev-parse", "--show-toplevel" })
    if not top then
        return nil
    end
    return vim.uv.fs_realpath(top) or top
end

-- project is the git common directory that holds dir, nil outside git.
local function project(dir)
    local common = git(dir, { "rev-parse", "--path-format=absolute",
        "--git-common-dir" })
    if not common then
        return nil
    end
    return vim.uv.fs_realpath(common) or common
end

-- branch is the branch dir has checked out, nil outside git and when the
-- branch is detached.
local function branch(dir)
    return git(dir, { "symbolic-ref", "--short", "-q", "HEAD" })
end

-- shorten shows dir with the home directory as ~.
local function shorten(dir)
    local home = vim.uv.os_homedir()
    if not home or home == "" then
        return dir
    end
    if dir == home then
        return "~"
    end
    if dir:sub(1, #home + 1) == home .. "/" then
        return "~" .. dir:sub(#home + 1)
    end
    return dir
end

-- order sorts recipients by how close they work to the worktree: in
-- it first, then in its project, then in the Rex session the plugin runs
-- in, then the rest.
local function order(recipients, worktree)
    local wanted = root(worktree)
    local common = project(worktree)
    local here = vim.env.REX_SESSION or ""
    local function rank(recipient)
        local dir = recipient.cwd
        if dir and wanted and root(dir) == wanted then
            return 1
        end
        if dir and common and project(dir) == common then
            return 2
        end
        if here ~= "" and recipient.session == here then
            return 3
        end
        return 4
    end
    local ranked = {}
    for at, recipient in ipairs(recipients) do
        ranked[at] = { at = at, rank = rank(recipient),
            recipient = recipient }
    end
    table.sort(ranked, function(a, b)
        if a.rank ~= b.rank then
            return a.rank < b.rank
        end
        return a.at < b.at
    end)
    for at, entry in ipairs(ranked) do
        recipients[at] = entry.recipient
    end
end

-- labels returns one line per recipient: aligned columns for the
-- harness, where it works, its branch, its state and its location, and
-- no id.
local function labels(recipients)
    local rows = {}
    for _, recipient in ipairs(recipients) do
        local current = recipient.cwd and branch(recipient.cwd) or nil
        rows[#rows + 1] = {
            recipient.harness or "",
            recipient.cwd and shorten(recipient.cwd) or "",
            current or "",
            recipient.state or "",
            recipient.location,
        }
    end
    local widths = {}
    for _, row in ipairs(rows) do
        for column = 1, 4 do
            widths[column] = math.max(widths[column] or 0, #row[column])
        end
    end
    local labels = {}
    for _, row in ipairs(rows) do
        local cells = {}
        for column = 1, 4 do
            cells[column] = row[column]
                .. string.rep(" ", widths[column] - #row[column])
        end
        if row[5] then
            cells[5] = "[" .. row[5] .. "]"
        end
        labels[#labels + 1] = (table.concat(cells, "  "):gsub("%s+$", ""))
    end
    return labels
end

-- collect returns the recipients of notifier name, ordered and
-- labelled for the picker.
local function collect(name, worktree)
    local notifier = require("revisa.notify." .. name)
    local found = notifier.recipients(worktree)
    if not found then
        return nil
    end
    order(found, worktree)
    local lines = labels(found)
    for at, recipient in ipairs(found) do
        recipient.label = lines[at]
    end
    return found
end

-- pick offers every recipient in order. Cancelling picks nothing.
local function pick(recipients, onChoice)
    local lines = {}
    for at, recipient in ipairs(recipients) do
        lines[at] = recipient.label
    end
    vim.ui.select(lines, { prompt = pickPrompt }, function(_, index)
        if index == nil then
            return
        end
        onChoice(recipients[index])
    end)
end

-- sent names the recipient in the "sent" notice: what runs there, and
-- where it sits.
local function sent(recipient)
    local where = recipient.location
    if not where and recipient.cwd then
        where = shorten(recipient.cwd)
    end
    local name = recipient.harness or recipient.id
    if where then
        name = name .. " in " .. where
    end
    vim.notify("revisa: sent to " .. name, vim.log.levels.INFO)
end

-- confirm asks before typing into the recipient: rex types into whatever
-- runs in the pane, so the pick stays explicit.
local function confirm(recipient, onChoice)
    vim.ui.select({ "send", "cancel" }, {
        prompt = "revisa: send to " .. recipient.label .. "?",
    }, function(_, index)
        if index == 1 then
            onChoice()
        end
    end)
end

local function offer(notifier, recipients, branch)
    local text = message(branch)
    pick(recipients, function(chosen)
        local function deliver()
            local result = notifier.deliver(chosen, text, branch)
            if result == "retry" then
                offer(notifier, recipients, branch)
                return
            end
            if result then
                cli.error(result)
                return
            end
            sent(chosen)
        end
        if notifier.confirm then
            confirm(chosen, deliver)
            return
        end
        deliver()
    end)
end

-- chosen is the notifier to use: the setting when it names one, else rex
-- where it can work, else recado, else nil and what to install.
local function chosen()
    if settings.notify ~= "auto" then
        return settings.notify, nil
    end
    if (vim.env.REX_SESSION or "") ~= ""
        and vim.fn.executable("rex") == 1 then
        return "rex", nil
    end
    if vim.fn.executable("recado") == 1 then
        return "recado", nil
    end
    return nil, noNotifier
end

function M.send(request)
    local name, why = chosen()
    if not name then
        cli.error(why)
        return
    end
    if name == "none" then
        cli.error(offHint)
        return
    end
    local notifier = require("revisa.notify." .. name)
    local ready, missing = notifier.available()
    if not ready then
        cli.error(missing)
        return
    end
    local found = target.of(api.nvim_get_current_buf())
    local dir, named = review.cliDir(request)
    local worktreePath = named
    if worktreePath == nil or worktreePath == "" then
        worktreePath = found and found.worktree
    end
    if not worktreePath then
        cli.error("revisa: :Revisa send needs a worktree, pass one")
        return
    end
    local args = { "changes", "--json", "--worktree", worktreePath }
    local data = cli.json({ cwd = dir }, args)
    if not data then
        return
    end
    if data.branch == nil or data.branch == "" then
        cli.error("revisa: " .. data.worktree .. " is not on a branch")
        return
    end
    local found = collect(name, data.worktree)
    if not found then
        return
    end
    if #found == 0 then
        cli.error("revisa: no recipient to notify")
        return
    end
    offer(notifier, found, data.branch)
end

return M
