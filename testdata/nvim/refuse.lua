local messages = H.notifications()
local kind = vim.env.REVISA_CASE
local named = {
    scheme = "tool",
    fugitive = "fugitive",
    diffview = "diffview",
}
if kind == "modified" then
    H.open()
    vim.api.nvim_buf_set_lines(0, 0, 1, false, { "unsaved" })
elseif kind == "virtual" or kind == "patch" then
    vim.api.nvim_buf_set_name(0, vim.env.REVISA_FILE)
    vim.bo.buftype = "nofile"
    if kind == "patch" then
        vim.bo.filetype = "diff"
    end
elseif named[kind] then
    vim.api.nvim_buf_set_name(0,
        named[kind] .. "://old-side/src/a.lua")
elseif kind == "missing" then
    vim.api.nvim_buf_set_name(0, vim.env.REVISA_FILE .. ".missing")
elseif kind == "directory" then
    vim.api.nvim_buf_set_name(0, vim.fs.dirname(vim.env.REVISA_FILE))
end
vim.cmd("Revisa comment")
local expected = vim.env.REVISA_ERROR
if expected and expected ~= "" then
    H.equal(expected, messages[#messages].message)
    H.equal(vim.log.levels.ERROR, messages[#messages].level)
else
    H.error(messages, "require")
end
for _, win in ipairs(vim.api.nvim_list_wins()) do
    H.equal("", vim.api.nvim_win_get_config(win).relative)
end
H.equal({}, H.marks(vim.api.nvim_get_current_buf()))
