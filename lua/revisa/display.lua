local M = {}
local api = vim.api
local namespace = api.nvim_create_namespace("revisa.comments")
local drawn = {}

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

local function shown(comment, count, show_resolved)
    -- Outdated and unknown comments have no current lines to draw on.
    if comment.side ~= "new" or type(comment.line_end) ~= "number"
        or comment.line_end > count
    then
        return false
    end
    if comment.status == "resolved" then
        return show_resolved
    end
    return comment.status == "open"
end

local function body_lines(comment)
    local lines = {}
    local function add(text, group)
        for _, line in ipairs(vim.split(text, "\n", { plain = true })) do
            lines[#lines + 1] = { { line, group } }
        end
    end
    add(comment.body, "RevisaComment")
    if type(comment.reply) == "string" then
        local reply = vim.split(comment.reply, "\n", { plain = true })
        reply[1] = "reply: " .. reply[1]
        for _, line in ipairs(reply) do
            lines[#lines + 1] = { { line, "RevisaReply" } }
        end
    end
    return lines
end

-- draw shows the comments of the file version target names. Only a
-- new-side target draws them: an old-side one has none to show.
function M.draw(buf, target, comments, show_resolved)
    api.nvim_buf_clear_namespace(buf, namespace, 0, -1)
    if target.side ~= "new" then
        return
    end
    drawn[buf] = true
    local count = api.nvim_buf_line_count(buf)
    for _, comment in ipairs(comments) do
        if comment.path == target.path
            and shown(comment, count, show_resolved)
        then
            local sign, sign_group = ">", "RevisaSign"
            if comment.status == "resolved" then
                sign, sign_group = "✓", "RevisaResolved"
            end
            for line = comment.line_start, comment.line_end do
                api.nvim_buf_set_extmark(buf, namespace, line - 1, 0, {
                    sign_text = sign,
                    sign_hl_group = sign_group,
                })
            end
            -- Leave room for padding below the anchor, even on empty lines.
            api.nvim_buf_set_extmark(buf, namespace, comment.line_end, 0, {
                virt_lines = body_lines(comment),
                virt_lines_above = true,
                right_gravity = false,
            })
        end
    end
end

return M
