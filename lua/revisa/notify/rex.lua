local M = {}
local cli = require("revisa.cli")
local notify = require("revisa.notify")

-- unavailable is the one reason every rex feature reports: both the
-- session it runs in and the executable are required.
local unavailable = "revisa: rex needs REX_SESSION set and rex on PATH"

-- previewLines bounds the capture the confirm shows.
local previewLines = 10

-- session is the rex session the plugin runs in, empty outside rex.
local function session()
    return vim.env.REX_SESSION or ""
end

-- M.run runs one rex command and returns its output, or nil and the
-- message to show. The command and its arguments are one word each.
function M.run(args)
    local command = { "rex" }
    vim.list_extend(command, args)
    return cli.exec(command, {}, "rex")
end

-- decode runs one rex command and parses its JSON output.
local function decode(args)
    local output, why = M.run(args)
    if not output then
        return nil, why
    end
    return cli.decode(output, "rex")
end

-- M.available reports whether rex can open a pane or take a message
-- from here.
function M.available()
    if session() == "" or vim.fn.executable("rex") ~= 1 then
        return false, unavailable
    end
    return true
end

-- cwd is where the program in block works, nil when rex cannot say. A
-- block listing carries no directory, so the pane's own process reports
-- it; the program in the foreground moves with the agent.
local function cwd(id)
    local data = decode({ "block", "call", "-s", session(), "-b", id,
        "process" })
    if not data then
        return nil
    end
    local processes = { data.foreground, data.child }
    for _, process in ipairs(processes) do
        if type(process) == "table" and type(process.cwd) == "string" then
            return process.cwd
        end
    end
    return nil
end

-- label is one picker entry: the block, its label and where it works.
local function label(block, dir)
    local parts = { block.block_id }
    if type(block.label) == "string" and block.label ~= "" then
        parts[#parts + 1] = block.label
    end
    if dir then
        parts[#parts + 1] = dir
    end
    return table.concat(parts, "  ")
end

-- M.targets lists the panes of the session a message can be typed into.
-- The pane the plugin runs in is left out: typing the message there puts
-- it in this editor, and a review there duplicates this one.
function M.targets(worktree)
    local data, why = decode({ "block", "ls", "-s", session(), "--json" })
    if not data then
        cli.error(why)
        return nil
    end
    local targets = {}
    for _, block in ipairs(data.blocks or {}) do
        if block.block_id ~= vim.env.REX_BLOCK then
            local dir = cwd(block.block_id)
            targets[#targets + 1] = {
                id = block.block_id,
                label = label(block, dir),
                cwd = dir,
            }
        end
    end
    for _, found in ipairs(notify.matched(targets, worktree)) do
        found.match = true
    end
    return targets
end

-- M.inspect returns what the pane shows now, in at most previewLines
-- lines. Rex types into whatever runs there, so the reviewer sees it
-- before confirming.
function M.inspect(target)
    local output, why = M.run({ "capture", "-s", session(), "-b", target.id,
        "--trim" })
    if not output then
        cli.error(why)
        return nil
    end
    local lines = vim.split(output, "\n", { plain = true })
    while #lines > 0 and lines[#lines] == "" do
        lines[#lines] = nil
    end
    while #lines > previewLines do
        table.remove(lines, 1)
    end
    return table.concat(lines, "\n")
end

-- M.deliver types the message into the pane and presses Return.
function M.deliver(target, message)
    local _, why = M.run({ "send", "-s", session(), "-b", target.id, message })
    if why then
        return why
    end
    _, why = M.run({ "send-key", "-s", session(), "-b", target.id, "enter" })
    if why then
        return why
    end
    vim.notify("revisa: sent to " .. target.id, vim.log.levels.INFO)
    return nil
end

return M
