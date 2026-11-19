local api = vim.api

-- A comment on the old side of a review belongs to the branch under
-- review, not to the branch the old side is read at or the one the
-- reviewer's worktree has checked out.

local function find(buf, text)
    for line, current in ipairs(api.nvim_buf_get_lines(buf, 0, -1, false)) do
        if current == text then
            return line
        end
    end
    error("no line " .. text)
end

require("revisa").review(vim.env.REVISA_TARGET)
local list = api.nvim_get_current_buf()
api.nvim_win_set_cursor(0, { find(list, "M src/a.lua"), 0 })
return function()
    H.keys("<CR>")
    return function()
        vim.cmd("2Revisa comment")
        local editor = H.float()
        api.nvim_buf_set_lines(editor, 0, -1, false, { "Old side body" })
        vim.cmd("write")
    end
end
