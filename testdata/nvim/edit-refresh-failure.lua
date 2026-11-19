local api = vim.api
local buf, win = H.open()
local marks = H.marks(buf)
api.nvim_win_set_cursor(win, { 2, 0 })
local selected = false
vim.ui.select = function()
    selected = true
end

-- Keep the last draw, but make the next real CLI list fail.
local result = vim.system({
    "git", "-C", vim.env.REVISA_WORKTREE, "checkout", "-q", "--detach",
}, { text = true }):wait()
H.equal(0, result.code)

local messages = H.notifications()
vim.cmd("Revisa edit")
H.equal(1, #messages)
H.error(messages, "HEAD is detached, pass --branch")
H.equal(false, selected)
H.equal(1, #api.nvim_list_wins())
H.equal(win, api.nvim_get_current_win())
H.equal(buf, api.nvim_get_current_buf())
H.equal(marks, H.marks(buf))
