local M = {}

local function line(text, group)
    return { { text, group } }
end

-- wrap breaks one line of text into lines of at most width cells. A
-- break goes after the last space that fits, else inside the word; the
-- spaces at a break are dropped, and the leading indent is kept.
local function wrap(text, width)
    local out = {}
    local chars = vim.fn.split(text, "\\zs")
    while vim.fn.strdisplaywidth(table.concat(chars)) > width do
        local used, fit, space = 0, 0, nil
        for index, char in ipairs(chars) do
            used = used + vim.fn.strdisplaywidth(char)
            if used > width then
                break
            end
            fit = index
            if char == " " and chars[index - 1] ~= nil
                and chars[index - 1] ~= " "
            then
                space = index
            end
        end
        local stop, start = math.max(fit, 1), math.max(fit, 1) + 1
        if chars[fit + 1] == " " then
            stop = fit
        elseif space then
            stop, start = space - 1, space + 1
        end
        out[#out + 1] = (table.concat(chars, "", 1, stop):gsub(" +$", ""))
        while chars[start] == " " do
            start = start + 1
        end
        chars = vim.list_slice(chars, start)
    end
    out[#out + 1] = table.concat(chars)
    return out
end

local function lines(text, group, width, prefix)
    local out = {}
    if type(text) ~= "string" then
        return out
    end
    local split = vim.split(text, "\n", { plain = true })
    split[1] = (prefix or "") .. split[1]
    for _, current in ipairs(split) do
        for _, wrapped in ipairs(wrap(current, width)) do
            out[#out + 1] = line(wrapped, group)
        end
    end
    return out
end

local function body_lines(comment, width)
    local out = lines(comment.body, "RevisaComment", width)
    if type(comment.reply) == "string" then
        vim.list_extend(out,
            lines(comment.reply, "RevisaReply", width, "reply: "))
    end
    return out
end

local function was_lines(comment, width)
    local out = {}
    if type(comment.anchor_text) ~= "string" then
        return out
    end
    for _, current in ipairs(vim.split(comment.anchor_text, "\n",
        { plain = true }))
    do
        vim.list_extend(out, lines(current, "RevisaWas", width, "was: "))
    end
    return out
end

local function anchored(comment, resolved, width)
    local placed = {
        kind = "anchored",
        sign = ">",
        group = "RevisaSign",
        line_start = comment.line_start,
        line_end = comment.line_end,
        body = body_lines(comment, width),
    }
    if resolved then
        placed.sign, placed.group = "✓", "RevisaResolved"
    end
    return placed
end

-- place says where one comment goes in the file version target shows.
-- line_count is the buffer's line count, width the cells a virtual line may
-- take, and with_resolved whether resolved comments are drawn. The rules
-- apply in order, and a comment with no current lines is hidden, never
-- clamped to the file. The CLI lists one file's comments, so place never
-- sees another file's.
function M.place(comment, target, line_count, with_resolved, width)
    if comment.side ~= target.side then
        return { kind = "hidden" }
    end
    if target.side == "old" and comment.commit ~= target.commit then
        return { kind = "hidden" }
    end
    if comment.status == "resolved" and not with_resolved then
        return { kind = "hidden" }
    end
    if comment.condition == "outdated" then
        return {
            kind = "marker",
            body = body_lines(comment, width),
            was = was_lines(comment, width),
        }
    end
    if type(comment.line_end) ~= "number" or comment.line_end > line_count
    then
        return { kind = "hidden" }
    end
    return anchored(comment, comment.status == "resolved", width)
end

return M
