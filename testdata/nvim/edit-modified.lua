local api = vim.api
local messages = H.notifications()
local buf = H.open()
api.nvim_buf_set_lines(buf, 0, 0, false, { "unsaved" })
api.nvim_win_set_cursor(0, { 2, 0 })
vim.cmd("Revisa edit")
H.error(messages, "save the buffer before editing")
for _, win in ipairs(api.nvim_list_wins()) do
    H.equal("", api.nvim_win_get_config(win).relative)
end
