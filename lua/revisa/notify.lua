local M = {}
local api = vim.api
local cli = require("revisa.cli")
local review = require("revisa.review")
local target = require("revisa.target")

local settings = { notify = "none" }
local allowed = { none = true, recado = true, rex = true }

local pickPrompt = "revisa: send the comments to"

local matchSuffix = "  (matches this worktree)"

function M.setup(options)
    local notify = options.notify
    if notify == nil then
        return
    end
    if not allowed[notify] then
        cli.error("revisa: notify must be none, recado or rex, got "
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

local function root(dir)
    local ok, result = pcall(function()
        return vim.system({
            "git", "-C", dir, "rev-parse", "--show-toplevel",
        }, { text = true }):wait()
    end)
    if not ok or result.code ~= 0 then
        return nil
    end
    local top = vim.trim(result.stdout or "")
    if top == "" then
        return nil
    end
    return vim.uv.fs_realpath(top) or top
end

-- M.matched returns the targets that work in worktree. A target's cwd is
-- resolved to the root of the worktree that holds it, so a subdirectory
-- of the worktree matches too.
function M.matched(targets, worktree)
    local want = vim.uv.fs_realpath(worktree) or worktree
    local matches = {}
    for _, candidate in ipairs(targets) do
        local found = candidate.cwd and root(candidate.cwd)
        if found and found == want then
            matches[#matches + 1] = candidate
        end
    end
    return matches
end

-- M.pick offers every target. The one target that works in the worktree
-- comes first and says so; several matches leave the list unmarked.
-- Cancelling picks nothing.
function M.pick(targets, prompt, onChoice)
    local matches = {}
    for _, candidate in ipairs(targets) do
        if candidate.match then
            matches[#matches + 1] = candidate
        end
    end
    local first = nil
    if #matches == 1 then
        first = matches[1]
    end
    local order, labels = {}, {}
    if first then
        order[1] = first
        labels[1] = first.label .. matchSuffix
    end
    for _, candidate in ipairs(targets) do
        if candidate ~= first then
            order[#order + 1] = candidate
            labels[#labels + 1] = candidate.label
        end
    end
    vim.ui.select(labels, { prompt = prompt }, function(_, index)
        if index == nil then
            return
        end
        onChoice(order[index])
    end)
end

-- confirm shows what the target holds and asks before typing into it. Rex
-- types into whatever runs in the pane, so the pick stays explicit.
local function confirm(target, preview, onChoice)
    if preview ~= "" then
        vim.notify(preview, vim.log.levels.INFO)
    end
    vim.ui.select({ "send", "cancel" }, {
        prompt = "revisa: send to " .. target.label .. "?",
    }, function(_, index)
        if index == 1 then
            onChoice()
        end
    end)
end

local function offer(notifier, targets, branch)
    local text = message(branch)
    M.pick(targets, pickPrompt, function(chosen)
        local function deliver()
            local result = notifier.deliver(chosen, text, branch)
            if result == "retry" then
                offer(notifier, targets, branch)
                return
            end
            if result then
                cli.error(result)
            end
        end
        if not notifier.inspect then
            deliver()
            return
        end
        local preview = notifier.inspect(chosen)
        if preview == nil then
            return
        end
        confirm(chosen, preview, deliver)
    end)
end

function M.send(request)
    local name = settings.notify
    if name == "none" then
        cli.error(
            'revisa: :Revisa send needs setup({ notify = "recado" | "rex" })')
        return
    end
    local notifier = require("revisa.notify." .. name)
    local ready, why = notifier.available()
    if not ready then
        cli.error(why)
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
    local targets = notifier.targets(data.worktree)
    if not targets then
        return
    end
    if #targets == 0 then
        cli.error("revisa: no pane to notify")
        return
    end
    offer(notifier, targets, data.branch)
end

return M
