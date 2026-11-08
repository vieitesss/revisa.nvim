local M = {}

-- line is one virtual line holding one chunk.
local function line(text, group)
    return { { text, group } }
end

-- lines splits text into virtual lines of one highlight group.
local function lines(text, group)
    local out = {}
    if type(text) ~= "string" then
        return out
    end
    for _, current in ipairs(vim.split(text, "\n", { plain = true })) do
        out[#out + 1] = line(current, group)
    end
    return out
end

-- body_lines is the comment body, then its reply when it has one.
local function body_lines(comment)
    local out = lines(comment.body, "RevisaComment")
    if type(comment.reply) == "string" then
        local reply = lines(comment.reply, "RevisaReply")
        reply[1][1][1] = "reply: " .. reply[1][1][1]
        vim.list_extend(out, reply)
    end
    return out
end

-- was_lines is the anchor text, as a marker shows it instead of a line.
local function was_lines(comment)
    local out = {}
    if type(comment.anchor_text) ~= "string" then
        return out
    end
    for _, current in ipairs(vim.split(comment.anchor_text, "\n",
        { plain = true }))
    do
        out[#out + 1] = line("was: " .. current, "RevisaWas")
    end
    return out
end

local function hidden(why)
    return { kind = "hidden", why = why }
end

local function anchored(comment, resolved)
    local placed = {
        kind = "anchored",
        sign = ">",
        group = "RevisaSign",
        line_start = comment.line_start,
        line_end = comment.line_end,
        body = body_lines(comment),
    }
    if resolved then
        placed.sign, placed.group = "✓", "RevisaResolved"
    end
    return placed
end

-- place says where one comment goes in the file version target shows.
-- line_count is the buffer's line count. The rules apply in order, and a
-- comment with no current lines is hidden, never clamped to the file.
function M.place(comment, target, line_count, show_resolved)
    if not target or target.kind ~= "file" then
        return hidden("not a file target")
    end
    if comment.path ~= target.path then
        return hidden("another file")
    end
    if comment.side ~= target.side then
        return hidden("another side")
    end
    if target.side == "old" and comment.commit ~= target.commit then
        return hidden("another commit")
    end
    if comment.status == "resolved" and not show_resolved then
        return hidden("resolved hidden")
    end
    if comment.condition == "outdated" or comment.condition == "unknown" then
        return {
            kind = "marker",
            condition = comment.condition,
            body = body_lines(comment),
            was = was_lines(comment),
        }
    end
    if type(comment.line_end) ~= "number" or comment.line_end > line_count
    then
        return hidden("no current lines")
    end
    return anchored(comment, comment.status == "resolved")
end

return M
