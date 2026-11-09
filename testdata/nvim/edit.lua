-- :Revisa edit opens the editor holding the body of the comment on the
-- cursor line, and saving stores the new body. Off any comment, it picks
-- one of the file's comments.
local api = vim.api
local buf = H.open()
api.nvim_win_set_cursor(0, { 2, 0 })
vim.cmd("Revisa edit")
local editor = H.float()
H.equal({ "First", "line" }, api.nvim_buf_get_lines(editor, 0, -1, false))
H.equal(false, vim.bo[editor].modified)
api.nvim_buf_set_lines(editor, 0, -1, false, { "First, edited" })
vim.cmd("write")
local shown = {}
for _, block in ipairs(H.virtual(buf)) do
    shown[#shown + 1] = block.lines[1][1][1]
end
H.equal(true, vim.tbl_contains(shown, "First, edited"))

local offered
vim.ui.select = function(items, options, choose)
    offered = {}
    for _, item in ipairs(items) do
        offered[#offered + 1] = options.format_item(item)
    end
    choose(items[2])
end
api.nvim_win_set_cursor(0, { 1, 0 })
vim.cmd("Revisa edit")
H.equal({ "line 2: First, edited", "lines 3-4: Second" }, offered)
editor = H.float()
H.equal({ "Second" }, api.nvim_buf_get_lines(editor, 0, -1, false))
vim.cmd("normal q")

local messages = H.notifications()
vim.cmd("enew")
vim.cmd("Revisa edit")
H.equal(true, #messages > 0)
