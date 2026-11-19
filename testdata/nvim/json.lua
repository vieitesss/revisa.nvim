-- Malformed CLI output reports one error per public command and leaves
-- the reviewer's file and existing comments intact.
local api = vim.api
local buf, win = H.open()
H.display(buf, 2, 2, { "Keep existing" })
vim.env.PATH = vim.env.REVISA_STUBS .. ":" .. vim.env.PATH
require("revisa").setup({ notify = "recado" })
local messages = H.notifications()

for _, command in ipairs({ "refresh", "review", "send" }) do
    local before = #messages
    vim.cmd("Revisa " .. command)
    H.equal(before + 1, #messages)
    H.error(messages, "revisa: invalid JSON from CLI: ")
    H.equal(win, api.nvim_get_current_win())
    H.equal(buf, api.nvim_get_current_buf())
    H.equal(1, #api.nvim_list_wins())
    H.display(buf, 2, 2, { "Keep existing" })
end
