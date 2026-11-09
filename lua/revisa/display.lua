local M = {}
local api = vim.api
local placement = require("revisa.placement")
local namespace = api.nvim_create_namespace("revisa.comments")
-- drawn holds, for each buffer drawn into, what its last draw showed, so a
-- resize can redraw it without asking the CLI again.
local drawn = {}

-- width is how many cells a comment line may take in buf: the text width
-- of the narrowest window showing it, kept between 20 and 80.
local function width(buf)
    local cells = 80
    for _, win in ipairs(vim.fn.win_findbuf(buf)) do
        local text = api.nvim_win_get_width(win)
            - vim.fn.getwininfo(win)[1].textoff
        cells = math.min(cells, text)
    end
    return math.max(20, cells)
end

-- buffers lists the loaded buffers this feature has drawn comments into.
function M.buffers()
    local bufs = {}
    for buf in pairs(drawn) do
        if api.nvim_buf_is_loaded(buf) then
            bufs[#bufs + 1] = buf
        else
            drawn[buf] = nil
        end
    end
    return bufs
end

-- groups is the order the end block groups markers in, with the highlight
-- group of each heading.
local groups = {
    { condition = "outdated", group = "RevisaOutdated" },
    { condition = "unknown", group = "RevisaUnknown" },
}

-- anchored draws a comment on its lines: a sign on each, the body below.
local function anchored(buf, placed)
    for line = placed.line_start, placed.line_end do
        api.nvim_buf_set_extmark(buf, namespace, line - 1, 0, {
            sign_text = placed.sign,
            sign_hl_group = placed.group,
        })
    end
    -- Leave room for padding below the anchor, even on empty lines.
    api.nvim_buf_set_extmark(buf, namespace, placed.line_end, 0, {
        virt_lines = placed.body,
        virt_lines_above = true,
        right_gravity = false,
    })
end

-- block groups the markers by condition into one block of virtual lines.
-- counts holds how many markers each condition has.
local function block(markers)
    local counts = {}
    for _, group in ipairs(groups) do
        counts[group.condition] = 0
    end
    for _, placed in ipairs(markers) do
        counts[placed.condition] = counts[placed.condition] + 1
    end
    local lines = {}
    for _, group in ipairs(groups) do
        local count = counts[group.condition]
        if count > 0 then
            if #lines > 0 then
                lines[#lines + 1] = { { "", "RevisaComment" } }
            end
            lines[#lines + 1] = {
                { group.condition .. " (" .. count .. ")", group.group },
            }
            for _, placed in ipairs(markers) do
                if placed.condition == group.condition then
                    vim.list_extend(lines, placed.body)
                    vim.list_extend(lines, placed.was)
                end
            end
        end
    end
    return lines, counts
end

-- summary is the line-1 text that makes the block findable in a long file.
local function summary(counts)
    local parts = {}
    for _, group in ipairs(groups) do
        local count = counts[group.condition]
        if count > 0 then
            parts[#parts + 1] = count .. " " .. group.condition
        end
    end
    if #parts == 0 then
        return nil
    end
    return "revisa: " .. table.concat(parts, ", ")
end

-- draw shows the comments of the file version target names. A comment
-- with no current lines is drawn in one block below the last line, not on
-- a guessed line.
function M.draw(buf, target, comments, show_resolved)
    api.nvim_buf_clear_namespace(buf, namespace, 0, -1)
    if not target or target.kind ~= "file" then
        drawn[buf] = nil
        return
    end
    drawn[buf] = {
        target = target, comments = comments, show_resolved = show_resolved,
    }
    local count = api.nvim_buf_line_count(buf)
    local cells = width(buf)
    local markers = {}
    for _, comment in ipairs(comments) do
        local placed = placement.place(comment, target, count, show_resolved,
            cells)
        if placed.kind == "anchored" then
            anchored(buf, placed)
        elseif placed.kind == "marker" then
            markers[#markers + 1] = placed
        end
    end
    local lines, counts = block(markers)
    if #lines == 0 then
        return
    end
    -- virt_lines_above on the line past the end is what renders below the
    -- last line: the same extmark without it is stored but never drawn.
    api.nvim_buf_set_extmark(buf, namespace, count, 0, {
        virt_lines = lines,
        virt_lines_above = true,
    })
    api.nvim_buf_set_extmark(buf, namespace, 0, 0, {
        virt_text = { { summary(counts), "RevisaOutdated" } },
        virt_text_pos = "right_align",
    })
end

-- redraw draws every loaded buffer again from its last draw, for a window
-- size change.
function M.redraw()
    for _, buf in ipairs(M.buffers()) do
        local last = drawn[buf]
        M.draw(buf, last.target, last.comments, last.show_resolved)
    end
end

return M
