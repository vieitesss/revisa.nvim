local M = {}
local cli = require("revisa.cli")
local notify = require("revisa.notify")

-- untrusted holds the harnesses whose reported cwd cannot be matched on.
local untrusted = { opencode = true }

-- remote is the prefix of an agent on another machine.
local remote = "recado://"

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

-- label is one picker entry: the agent, its harness and where it works.
local function label(agent)
    local parts = { agent.id }
    if type(agent.harness) == "string" and agent.harness ~= "" then
        parts[#parts + 1] = agent.harness
    end
    local dir = cwd(agent)
    if dir then
        parts[#parts + 1] = dir
    end
    return table.concat(parts, "  ")
end

-- M.available reports whether recado can carry a message from here.
function M.available()
    if vim.fn.executable("recado") ~= 1 then
        return false, "revisa: recado is not on PATH"
    end
    return true
end

-- M.targets lists the agents that can receive the message here. Only
-- live agents on this machine are offered: remote agents, and Codex,
-- which cannot receive these messages, are dropped.
function M.targets(worktree)
    local output, why = cli.exec({ "recado", "agents", "--json" }, {},
        "recado agents")
    if not output then
        cli.error(why)
        return nil
    end
    local data, bad = cli.decode(output, "recado")
    if type(data) ~= "table" then
        cli.error(bad or "revisa: invalid JSON from recado: "
            .. tostring(data))
        return nil
    end
    local targets = {}
    for _, agent in ipairs(data.agents or {}) do
        if receivable(agent) then
            targets[#targets + 1] = {
                id = agent.id,
                label = label(agent),
                cwd = cwd(agent),
            }
        end
    end
    if #targets == 0 then
        cli.error("revisa: no live agent to notify")
        return nil
    end
    for _, found in ipairs(notify.matched(targets, worktree)) do
        found.match = true
    end
    return targets
end

-- M.deliver carries the pointer to agent, one argument per word. Exit 2
-- means recado wants another agent, so the picker opens again.
function M.deliver(agent, message, branch)
    local ok, result = pcall(function()
        return vim.system({
            "recado", "send", agent.id, message,
            "--meta", "revisa_branch=" .. branch,
        }, { env = { RECADO_ADDR = "revisa" } }):wait()
    end)
    if not ok then
        return tostring(result)
    end
    if result.code == 0 then
        vim.notify("revisa: sent to " .. agent.id, vim.log.levels.INFO)
        return nil
    end
    if result.code == 2 then
        return "retry"
    end
    local text = vim.trim(result.stderr or "")
    if text == "" then
        text = "recado send failed (exit " .. result.code .. ")"
    end
    return text
end

return M
