local api = vim.api
local buf = api.nvim_get_current_buf()
local name = api.nvim_buf_get_name(buf)
local lines = api.nvim_buf_get_lines(buf, 0, -1, false)
local want = "worktree " .. vim.env.REVISA_EXPECT
if name ~= vim.env.REVISA_NAME or lines[1] ~= want then
    io.stderr:write("revisa review opened " .. name .. " with " ..
        tostring(lines[1]) .. "\nwant " .. vim.env.REVISA_NAME ..
        " with " .. want .. "\n")
    vim.cmd("cquit 1")
end
vim.cmd("qa!")
