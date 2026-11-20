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

-- named is the label a window was given, nil when rex made one up.
local function named(label)
    if type(label) ~= "string" or label == "" then
        return nil
    end
    if label:match(unnamedWindow) then
        return nil
    end
    return label
end

-- walk records where one layout node puts its block, and recurses into
-- the two sides of a split.
local function walk(node, floating, found)
    if type(node) ~= "table" then
        return
    end
    if node.block_id then
        found[node.block_id] = { floating = floating, rect = node.rect }
        return
    end
    local split = node.split
    if type(split) == "table" then
        walk(split.before, floating, found)
        walk(split.after, floating, found)
    end
end

-- sites maps every block of a window to where it sits: whether it floats
-- and the rect its layout node gives it. A layer that lists no layout
-- leaves the rect out.
local function sites(layers)
    local found = {}
    for _, layer in ipairs(layers or {}) do
        local floating = layer.kind == "floating"
        walk(layer.layout, floating, found)
        for _, block in ipairs(layer.blocks or {}) do
            if found[block.block_id] == nil then
                found[block.block_id] = { floating = floating }
            end
        end
    end
    return found
end

-- windows lists the windows of one session in tab-bar order, with the
-- label each was given and where its blocks sit.
local function windows(id)
    local data, why = decode({ "window", "ls", "-s", id, "--json" })
    if not data then
        return nil, why
    end
    local found = {}
    for at, window in ipairs(data.windows or {}) do
        found[#found + 1] = {
            id = window.window_id,
            at = at,
            name = named(window.label),
            sites = sites(window.layers),
        }
    end
    return found
end

-- ownWindow is the window the plugin runs in: the one block ls puts its
-- block in, or failing that the one window ls lists it in.
local function ownWindow(agents, listed)
    local mine = vim.env.REX_BLOCK
    if mine == nil or mine == "" then
        return nil
    end
    for _, block in ipairs(agents) do
        if block.block_id == mine then
            return block.window_id
        end
    end
    for _, window in ipairs(listed) do
        if window.sites[mine] then
            return window.id
        end
    end
    return nil
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

-- place names where one recipient sits: its session, then the window
-- when that window is worth naming. A window nobody named shows as its
-- tab when the session has more than one, or when the plugin runs in it.
-- The window the plugin runs in is marked here.
local function place(label, window, here, count)
    if label == "" or label == nil then
        return nil
    end
    if window == nil then
        return label
    end
    local name = window.name
    if name == nil then
        if count <= 1 and not here then
            return label
        end
        name = "tab " .. window.at
    end
    if here then
        name = name .. " (here)"
    end
    return label .. " › " .. name
end

-- distinct returns the distinct values one axis of the rects takes,
-- smallest first.
local function distinct(recipients, axis)
    local seen, list = {}, {}
    for _, recipient in ipairs(recipients) do
        local value = recipient.site.rect[axis]
        if not seen[value] then
            seen[value] = true
            list[#list + 1] = value
        end
    end
    table.sort(list)
    return list
end

-- slot is the 1-based place of value in list.
local function slot(list, value)
    for at, entry in ipairs(list) do
        if entry == value then
            return at
        end
    end
    return nil
end

-- words names one axis of a grid of up to three slots, nil when the
-- axis has a single slot or too many to name shortly.
local function words(count, low, middle, high)
    if count == 2 then
        return { low, high }
    end
    if count == 3 then
        return { low, middle, high }
    end
    return nil
end

-- positions names where each tiled recipient of one window sits, in the
-- order given, nil when their rects cannot name every one of them.
local function positions(recipients)
    for _, recipient in ipairs(recipients) do
        if type(recipient.site.rect) ~= "table" then
            return nil
        end
    end
    local columns = distinct(recipients, 1)
    local rows = distinct(recipients, 2)
    if #columns == 1 and #rows == 1 then
        return nil
    end
    local across = words(#columns, "left", "center", "right")
    local down = words(#rows, "top", "middle", "bottom")
    if (#columns > 1 and across == nil)
        or (#rows > 1 and down == nil) then
        return nil
    end
    local labels, seen = {}, {}
    for at, recipient in ipairs(recipients) do
        local parts = {}
        if down then
            parts[#parts + 1] = down[slot(rows, recipient.site.rect[2])]
        end
        if across then
            parts[#parts + 1] = across[slot(columns,
                recipient.site.rect[1])]
        end
        local label = table.concat(parts, "-")
        if seen[label] then
            return nil
        end
        seen[label] = true
        labels[at] = label
    end
    return labels
end

-- bounds is where one recipient sits, empty when its layout says
-- nothing.
local function bounds(recipient)
    local site = recipient.site or {}
    return site.rect or {}
end

-- bySite orders the recipients of one window left to right, then top to
-- bottom, keeping the order given when their rects say nothing.
local function bySite(recipients)
    local ordered = {}
    for at, recipient in ipairs(recipients) do
        ordered[at] = { at = at, recipient = recipient }
    end
    table.sort(ordered, function(a, b)
        local left, right = bounds(a.recipient), bounds(b.recipient)
        if (left[1] or 0) ~= (right[1] or 0) then
            return (left[1] or 0) < (right[1] or 0)
        end
        if (left[2] or 0) ~= (right[2] or 0) then
            return (left[2] or 0) < (right[2] or 0)
        end
        return a.at < b.at
    end)
    local sorted = {}
    for at, entry in ipairs(ordered) do
        sorted[at] = entry.recipient
    end
    return sorted
end

-- numbered labels tiled recipients that share a window when their rects
-- name no side: pane 1, pane 2, ... left to right, then top to bottom.
local function numbered(recipients)
    for at, recipient in ipairs(bySite(recipients)) do
        recipient.location = recipient.location .. " › pane " .. at
    end
end

-- suffixes tells apart the recipients that share one window: tiled
-- blocks get the side of the window they sit in, floating blocks say so.
local function suffixes(group)
    local tiled, floating = {}, {}
    for _, recipient in ipairs(group) do
        local site = recipient.site
        if site and site.floating then
            floating[#floating + 1] = recipient
        elseif site and type(site.rect) == "table"
            and type(site.rect[1]) == "number"
            and type(site.rect[2]) == "number" then
            tiled[#tiled + 1] = recipient
        end
    end
    floating = bySite(floating)
    for at, recipient in ipairs(floating) do
        local side = " › floating"
        if #floating > 1 then
            side = side .. " " .. at
        end
        recipient.location = recipient.location .. side
    end
    if #tiled < 2 then
        return
    end
    local labels = positions(tiled)
    if labels == nil then
        numbered(tiled)
        return
    end
    for at, recipient in ipairs(tiled) do
        recipient.location = recipient.location .. " › " .. labels[at]
    end
end

-- locate names where the recipients of one session sit, and tells apart
-- the ones that share a window.
local function locate(recipients, label, count)
    local groups = {}
    for _, recipient in ipairs(recipients) do
        recipient.location = place(label, recipient.window,
            recipient.here, count)
        if recipient.location and recipient.window then
            local group = groups[recipient.window.id]
            if group == nil then
                group = {}
                groups[recipient.window.id] = group
            end
            group[#group + 1] = recipient
        end
    end
    for _, group in pairs(groups) do
        if #group > 1 then
            suffixes(group)
        end
    end
end

-- closer orders the recipients of one session: the window the plugin
-- runs in first, then the other windows in tab-bar order, and within a
-- window the tiled blocks left to right, then top to bottom, then the
-- floating ones.
local function closer(recipients)
    local function key(recipient)
        local site = recipient.site or {}
        local rect = bounds(recipient)
        local window = math.huge
        if recipient.here then
            window = 0
        elseif recipient.window then
            window = recipient.window.at
        end
        return { window, site.floating and 1 or 0,
            rect[1] or 0, rect[2] or 0, recipient.at }
    end
    table.sort(recipients, function(a, b)
        local left, right = key(a), key(b)
        for at = 1, #left do
            if left[at] ~= right[at] then
                return left[at] < right[at]
            end
        end
        return false
    end)
end

-- M.recipients lists the blocks that run a program reporting itself, in
-- every session. The pane the plugin runs in is left out: typing the
-- message there puts it in this editor, and a review there duplicates
-- this one. The session the plugin runs in comes first, and inside a
-- session the recipients come in window and pane order, so a tie keeps
-- the nearest window first.
function M.recipients()
    local found, why = sessions()
    if not found then
        cli.error(why)
        return nil
    end
    local here = session()
    local ordered = {}
    for _, entry in ipairs(found) do
        if entry.id == here then
            table.insert(ordered, 1, entry)
        else
            ordered[#ordered + 1] = entry
        end
    end
    local recipients = {}
    for _, entry in ipairs(ordered) do
        local listed, bad = windows(entry.id)
        if not listed then
            cli.error(bad)
            return nil
        end
        local agents, failed = blocks(entry.id)
        if not agents then
            cli.error(failed)
            return nil
        end
        local own = ownWindow(agents, listed)
        local byID = {}
        for _, window in ipairs(listed) do
            byID[window.id] = window
        end
        local mine = {}
        for _, block in ipairs(agents) do
            if block.creator_name == terminal
                and block.block_id ~= vim.env.REX_BLOCK then
                local reports, gone = records(entry.id, block.block_id)
                if not reports then
                    cli.error(gone)
                    return nil
                end
                if #reports > 0 then
                    local running, stopped = program(entry.id,
                        block.block_id)
                    if not running then
                        cli.error(stopped)
                        return nil
                    end
                    local window = byID[block.window_id]
                    mine[#mine + 1] = {
                        id = block.block_id,
                        session = entry.id,
                        harness = running.harness,
                        cwd = running.cwd,
                        state = state(reports[1]),
                        window = window,
                        site = window and window.sites[block.block_id],
                        here = window ~= nil and window.id == own,
                        at = #mine + 1,
                    }
                end
            end
        end
        locate(mine, entry.label, #listed)
        closer(mine)
        for _, recipient in ipairs(mine) do
            recipients[#recipients + 1] = recipient
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
