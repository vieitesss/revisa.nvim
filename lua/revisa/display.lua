local M = {}
local api = vim.api
local placement = require("revisa.placement")
local namespace = api.nvim_create_namespace("revisa.comments")
local drawn = {}

local function width(buf)
    local cells = 80
    for _, win in ipairs(vim.fn.win_findbuf(buf)) do
        local text = api.nvim_win_get_width(win)
            - vim.fn.getwininfo(win)[1].textoff
        cells = math.min(cells, text)
    end
    return math.max(20, cells)
end

local function place(comment, where, moved)
    local now = moved[comment]
    if now and now.deleted then
        return { kind = "hidden", why = "lines deleted" }
    end
    if now then
        comment = vim.tbl_extend("force", comment, now)
    end
    return placement.place(comment, where.target, where.count,
        where.show_resolved, where.cells)
end

local function hasAutoSignColumn(win)
    local column = api.nvim_get_option_value("signcolumn", { win = win })
    return column == "auto" or column:sub(1, 5) == "auto:"
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

local groups = {
    { condition = "outdated", group = "RevisaOutdated" },
    { condition = "unknown", group = "RevisaUnknown" },
}

-- signs marks the lines of an anchored comment. It returns the extmarks
-- of the first and last signs, so a later redraw can read the lines they
-- moved to. A sign whose line is deleted comes back marked invalid, which
-- is how a redraw tells a deleted anchor from a moved one.
local function signs(buf, placed)
    local first, last
    for line = placed.line_start, placed.line_end do
        last = api.nvim_buf_set_extmark(buf, namespace, line - 1, 0, {
            sign_text = placed.sign,
            sign_hl_group = placed.group,
            invalidate = true,
        })
        first = first or last
    end
    return first, last
end

local function body(buf, placed)
    api.nvim_buf_set_extmark(buf, namespace, placed.line_end, 0, {
        virt_lines = placed.body,
        virt_lines_above = true,
        right_gravity = false,
    })
end

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

function M.draw(buf, target, comments, show_resolved, moved)
    api.nvim_buf_clear_namespace(buf, namespace, 0, -1)
    if not target or target.kind ~= "file" then
        drawn[buf] = nil
        return
    end
    local count = api.nvim_buf_line_count(buf)
    moved = moved or {}
    local deleted = {}
    for comment, now in pairs(moved) do
        if now.deleted then
            deleted[comment] = true
        end
    end
    local where = {
        target = target, count = count, show_resolved = show_resolved,
        cells = width(buf),
    }
    -- The signs come first: they widen the gutter when 'signcolumn' is
    -- auto, so the cells a body may take are measured once they are there.
    local placed = {}
    local anchors = {}
    for _, comment in ipairs(comments) do
        local one = place(comment, where, moved)
        placed[#placed + 1] = one
        if one.kind == "anchored" then
            local first, last = signs(buf, one)
            anchors[#anchors + 1] = {
                comment = comment, first = first, last = last,
            }
        end
    end
    drawn[buf] = {
        target = target, comments = comments, show_resolved = show_resolved,
        anchors = anchors, deleted = deleted,
    }
    -- A sign column that 'signcolumn' keeps hidden until a sign shows is
    -- laid out on the next redraw, so the bodies are wrapped after it.
    for _, win in ipairs(vim.fn.win_findbuf(buf)) do
        if hasAutoSignColumn(win) then
            vim.cmd.redraw()
            break
        end
    end
    local room = width(buf)
    if room ~= where.cells then
        where.cells = room
        for index, one in ipairs(placed) do
            if one.kind ~= "hidden" then
                placed[index] = place(comments[index], where, moved)
            end
        end
    end
    local markers = {}
    for _, one in ipairs(placed) do
        if one.kind == "anchored" then
            body(buf, one)
        elseif one.kind == "marker" then
            markers[#markers + 1] = one
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

local function row(buf, id, count)
    local mark = api.nvim_buf_get_extmark_by_id(buf, namespace, id,
        { details = true })
    if #mark == 0 or (mark[3] and mark[3].invalid) then
        return nil
    end
    local line = mark[1] + 1
    if line < 1 or line > count then
        return nil
    end
    return line
end

local function moved(buf, anchors)
    local count = api.nvim_buf_line_count(buf)
    local rows = {}
    for _, anchor in ipairs(anchors) do
        local first = row(buf, anchor.first, count)
        local last = row(buf, anchor.last, count)
        if first and last then
            rows[anchor.comment] = { line_start = first, line_end = last }
        else
            rows[anchor.comment] = { deleted = true }
        end
    end
    return rows
end

-- redraw draws every loaded buffer again from its last draw, for a window
-- size change. A comment deleted since the last draw stays hidden: only a
-- refresh from the CLI gives it a line again.
function M.redraw()
    for _, buf in ipairs(M.buffers()) do
        local last = drawn[buf]
        local now = moved(buf, last.anchors)
        for comment in pairs(last.deleted) do
            now[comment] = { deleted = true }
        end
        M.draw(buf, last.target, last.comments, last.show_resolved, now)
    end
end

return M
