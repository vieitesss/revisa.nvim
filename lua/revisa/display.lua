local M = {}
local api = vim.api
local namespace = api.nvim_create_namespace("revisa.comments")

function M.draw(buf, path, comments)
    api.nvim_buf_clear_namespace(buf, namespace, 0, -1)
    local count = api.nvim_buf_line_count(buf)
    for _, comment in ipairs(comments) do
        if comment.path == path and comment.side == "new"
            and comment.status == "open" and comment.line_end <= count
        then
            local body = {}
            for _, line in ipairs(vim.split(comment.body, "\n", {
                plain = true,
            })) do
                body[#body + 1] = { { line, "RevisaComment" } }
            end
            for line = comment.line_start, comment.line_end do
                local options = {
                    sign_text = ">",
                    sign_hl_group = "RevisaSign",
                }
                api.nvim_buf_set_extmark(buf, namespace, line - 1, 0, options)
            end
            -- Leave room for padding below the anchor, even on empty lines.
            api.nvim_buf_set_extmark(buf, namespace, comment.line_end, 0, {
                virt_lines = body,
                virt_lines_above = true,
                right_gravity = false,
            })
        end
    end
end

return M
