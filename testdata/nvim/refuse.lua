local messages = H.notifications()
local kind = vim.env.REVISA_CASE
if kind == "modified" then
    H.open()
    vim.api.nvim_buf_set_lines(0, 0, 1, false, { "unsaved" })
elseif kind == "virtual" or kind == "patch" then
    vim.api.nvim_buf_set_name(0, vim.env.REVISA_FILE)
    vim.bo.buftype = "nofile"
    if kind == "patch" then
        vim.bo.filetype = "diff"
    end
elseif kind == "scheme" then
    vim.api.nvim_buf_set_name(0, "tool://old-side/src/a.lua")
elseif kind == "missing" then
    vim.api.nvim_buf_set_name(0, vim.env.REVISA_FILE .. ".missing")
elseif kind == "directory" then
    vim.api.nvim_buf_set_name(0, vim.fs.dirname(vim.env.REVISA_FILE))
end
vim.cmd("Revisa comment")
H.error(messages, kind == "modified"
    and "save the buffer before commenting" or "require")
for _, win in ipairs(vim.api.nvim_list_wins()) do
    H.equal("", vim.api.nvim_win_get_config(win).relative)
end
H.equal({}, H.marks(vim.api.nvim_get_current_buf()))
