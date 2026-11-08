local M = {}
local api = vim.api
local cli = require("revisa.cli")
local target = require("revisa.target")

local settings = { notify = "none" }
local allowed = { none = true, recado = true, rex = true }

-- untrusted holds the harnesses whose reported cwd cannot be matched on.
local untrusted = { opencode = true }

-- remote is the prefix of an agent on another machine.
local remote = "recado://"

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

-- decode runs one CLI command in dir and parses its JSON output.
local function decode(dir, args)
    local output = cli.run({ cwd = dir }, args)
    if not output then
        return nil
    end
    local ok, data = pcall(vim.json.decode, output)
    if not ok then
        cli.error("revisa: invalid JSON from CLI: " .. tostring(data))
        return nil
    end
    return data
end

-- quote wraps a branch name in single quotes for a shell.
local function quote(text)
    return "'" .. text:gsub("'", "'\\''") .. "'"
end

-- message points the agent at the open comments and says where to answer.
local function message(branch)
    local quoted = quote(branch)
    return "revisa: review comments are waiting on branch " .. quoted
        .. ". Run: revisa list --branch " .. quoted .. " --status open."
        .. " Answer each with revisa resolve, not recado reply."
end

-- receivable reports whether recado may deliver this message to agent.
local function receivable(agent)
    if agent.live ~= true or type(agent.id) ~= "string" then
        return false
    end
    if agent.id:sub(1, #remote) == remote then
        return false
    end
    if agent.machine ~= nil or agent.remote ~= nil then
        return false
    end
    return agent.harness ~= "codex"
end

-- cwd returns the directory recado reports for agent, nil when it cannot
-- be trusted to say where the agent works.
local function cwd(agent)
    if untrusted[agent.harness] then
        return nil
    end
    if type(agent.cwd) ~= "string" or agent.cwd == "" then
        return nil
    end
    return agent.cwd
end

-- root resolves dir to the real path of the worktree that holds it.
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

-- matched returns the agents that work in worktree.
local function matched(agents, worktree)
    local target = vim.uv.fs_realpath(worktree) or worktree
    local matches = {}
    for _, agent in ipairs(agents) do
        local dir = cwd(agent)
        local found = dir and root(dir)
        if found and found == target then
            matches[#matches + 1] = agent
        end
    end
    return matches
end

-- label is one picker entry: the agent, its harness and where it works.
-- A match says so.
local function label(agent, match)
    local parts = { agent.id }
    if type(agent.harness) == "string" and agent.harness ~= "" then
        parts[#parts + 1] = agent.harness
    end
    local dir = cwd(agent)
    if dir then
        parts[#parts + 1] = dir
    end
    local text = table.concat(parts, "  ")
    if match then
        text = text .. "  (matches this worktree)"
    end
    return text
end

-- candidates lists the agents that can receive the message here.
local function candidates()
    local ok, result = pcall(function()
        return vim.system({ "recado", "agents", "--json" },
            { text = true }):wait()
    end)
    if not ok then
        cli.error(tostring(result))
        return nil
    end
    if result.code ~= 0 then
        local text = vim.trim(result.stderr or "")
        if text == "" then
            text = "recado agents failed (exit " .. result.code .. ")"
        end
        cli.error(text)
        return nil
    end
    local decoded, data = pcall(vim.json.decode, result.stdout)
    if not decoded or type(data) ~= "table" then
        cli.error("revisa: invalid JSON from recado: " .. tostring(data))
        return nil
    end
    local agents = {}
    for _, agent in ipairs(data.agents or {}) do
        if receivable(agent) then
            agents[#agents + 1] = agent
        end
    end
    if #agents == 0 then
        cli.error("revisa: no live agent to notify")
        return nil
    end
    return agents
end

-- pick offers every candidate. The one agent of worktree comes first and
-- says so; otherwise the list has no preselection. Cancelling sends
-- nothing.
local function pick(agents, worktree, onChoice)
    local matches = matched(agents, worktree)
    local first = nil
    if #matches == 1 then
        first = matches[1]
    end
    local order, labels = {}, {}
    if first then
        order[1] = first
        labels[1] = label(first, true)
    end
    for _, agent in ipairs(agents) do
        if agent ~= first then
            order[#order + 1] = agent
            labels[#labels + 1] = label(agent, false)
        end
    end
    vim.ui.select(labels, { prompt = "revisa: send the comments to" },
        function(_, index)
            if index == nil then
                return
            end
            onChoice(order[index])
        end)
end

-- deliver carries the pointer to agent, one argument per word. Exit 2
-- means recado wants another agent, so the picker opens again.
local function deliver(agent, branch, agents, worktree)
    local ok, result = pcall(function()
        return vim.system({
            "recado", "send", agent.id, message(branch),
            "--meta", "revisa_branch=" .. branch,
        }, { env = { RECADO_ADDR = "revisa" } }):wait()
    end)
    if not ok then
        cli.error(tostring(result))
        return
    end
    if result.code == 0 then
        vim.notify("revisa: sent to " .. agent.id, vim.log.levels.INFO)
        return
    end
    if result.code == 2 then
        pick(agents, worktree, function(next)
            deliver(next, branch, agents, worktree)
        end)
        return
    end
    local text = vim.trim(result.stderr or "")
    if text == "" then
        text = "recado send failed (exit " .. result.code .. ")"
    end
    cli.error(text)
end

-- M.send tells the agent of a worktree that comments are waiting there.
function M.send(request)
    if settings.notify ~= "recado" then
        cli.error('revisa: :Revisa send needs setup({ notify = "recado" })')
        return
    end
    if vim.fn.executable("recado") ~= 1 then
        cli.error("revisa: recado is not on PATH")
        return
    end
    local found = target.of(api.nvim_get_current_buf())
    local worktreePath = found and found.worktree
    if request and request ~= "" then
        worktreePath = request
    end
    if not worktreePath then
        cli.error("revisa: :Revisa send needs a worktree, pass one")
        return
    end
    local dir = found and found.cwd or vim.fn.getcwd()
    local args = { "changes", "--json", "--worktree", worktreePath }
    local data = decode(dir, args)
    if not data then
        return
    end
    if data.branch == nil or data.branch == "" then
        cli.error("revisa: " .. data.worktree .. " is not on a branch")
        return
    end
    local agents = candidates()
    if not agents then
        return
    end
    pick(agents, data.worktree, function(agent)
        deliver(agent, data.branch, agents, data.worktree)
    end)
end

return M
