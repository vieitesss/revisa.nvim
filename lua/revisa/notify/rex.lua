local M = {}
local cli = require("revisa.cli")

-- M.confirm asks before typing: rex types into whatever runs in a pane.
M.confirm = true

-- unnamedWindow is the label rex makes up for a window nobody named.
local unnamedWindow = "^Window %x+$"

-- terminal is the creator of the blocks rex can type into. Other blocks,
-- such as port forwards, answer no program_status call.
local terminal = "com.superlogical.terminal"

-- unavailable is the one reason every rex feature reports: both the
-- session it runs in and the executable are required.
local unavailable = "revisa: rex needs REX_SESSION set and rex on PATH"

-- session is the rex session the plugin runs in, empty outside rex.
local function session()
    return vim.env.REX_SESSION or ""
end

-- run runs one rex command and returns its output, or nil and the
-- message to show. The command and its arguments are one word each.
local function run(args)
    local command = { "rex" }
    vim.list_extend(command, args)
    return cli.exec(command, {}, "rex")
end

-- decode runs one rex command and parses its JSON output.
local function decode(args)
    local output, why = run(args)
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

-- sessions lists every rex session, as id and label.
local function sessions()
    local data, why = decode({ "session", "ls", "--json" })
    if not data then
        return nil, why
    end
    local found = {}
    for _, entry in ipairs(data.sessions or {}) do
        if type(entry.session_id) == "string" then
            found[#found + 1] = {
                id = entry.session_id,
                label = entry.label or "",
            }
        end
    end
    return found
end

-- blocks lists the blocks of one session.
local function blocks(id)
    local data, why = decode({ "block", "ls", "-s", id, "--json" })
    if not data then
        return nil, why
    end
    return data.blocks or {}
end

-- windows maps the windows of one session to their labels, leaving out
-- the label rex makes up for a window nobody named.
local function windows(id)
    local data, why = decode({ "window", "ls", "-s", id, "--json" })
    if not data then
        return nil, why
    end
    local labels = {}
    for _, window in ipairs(data.windows or {}) do
        local label = window.label
        if type(label) == "string" and not label:match(unnamedWindow) then
            labels[window.window_id] = label
        end
    end
    return labels
end

-- records is what the programs in one terminal block report about
-- themselves. A shell or an editor holds no program that reports itself
-- and answers with an empty list.
local function records(id, block)
    local data, why = decode({ "block", "call", "-s", id, "-b", block,
        "program_status" })
    if not data then
        return nil, why
    end
    return data.records or {}
end

-- state names what a program reports: busy while it works, idle when it
-- waits or finished, blank when it says something else.
local function state(record)
    if record.state == "working" then
        return "busy"
    end
    if record.state == "idle" or record.state == "done" then
        return "idle"
    end
    return nil
end

-- program is what runs in one block: the foreground program's name and
-- the directory it works in.
local function program(id, block)
    local data, why = decode({ "block", "call", "-s", id, "-b", block,
        "process" })
    if not data then
        return nil, why
    end
    local foreground = data.foreground or {}
    return { harness = foreground.argv0, cwd = foreground.cwd }
end

-- place names where a block sits: its session, then the window it is in
-- when that window has a label of its own.
local function place(label, window)
    if label == "" or label == nil then
        return nil
    end
    if window == nil then
        return label
    end
    return label .. " › " .. window
end

-- M.recipients lists the blocks that run a program reporting itself, in
-- every session. The pane the plugin runs in is left out: typing the
-- message there puts it in this editor, and a review there duplicates
-- this one.
function M.recipients()
    local found, why = sessions()
    if not found then
        cli.error(why)
        return nil
    end
    local recipients = {}
    for _, entry in ipairs(found) do
        local labels, bad = windows(entry.id)
        if not labels then
            cli.error(bad)
            return nil
        end
        local panes, failed = blocks(entry.id)
        if not panes then
            cli.error(failed)
            return nil
        end
        for _, block in ipairs(panes) do
            if block.creator_name == terminal
                and block.block_id ~= vim.env.REX_BLOCK then
                local listed, gone = records(entry.id, block.block_id)
                if not listed then
                    cli.error(gone)
                    return nil
                end
                if #listed > 0 then
                    local running, stopped = program(entry.id,
                        block.block_id)
                    if not running then
                        cli.error(stopped)
                        return nil
                    end
                    recipients[#recipients + 1] = {
                        id = block.block_id,
                        session = entry.id,
                        harness = running.harness,
                        cwd = running.cwd,
                        state = state(listed[1]),
                        location = place(entry.label,
                            labels[block.window_id]),
                    }
                end
            end
        end
    end
    return recipients
end

-- M.deliver types the message into the block and presses Return, in the
-- session that block belongs to.
function M.deliver(recipient, message)
    local _, why = run({ "send", "-s", recipient.session, "-b",
        recipient.id, message })
    if why then
        return why
    end
    _, why = run({ "send-key", "-s", recipient.session, "-b",
        recipient.id, "enter" })
    if why then
        return why
    end
    return nil
end

return M
