local api = vim.api
local worktree = vim.env.REVISA_LINKED

require("revisa.diff").open({
    worktree = worktree,
    branch = "feat/x",
    commit = vim.env.REVISA_BASE,
    status = "M",
    path = "src/a.lua",
})

local new
for _, win in ipairs(api.nvim_tabpage_list_wins(0)) do
    local name = api.nvim_buf_get_name(api.nvim_win_get_buf(win))
    if not name:find("^revisa://") then
        new = win
    end
end
assert(new, "no real-file side in the diff tab")
api.nvim_set_current_win(new)
vim.cmd.saveas(vim.fn.fnameescape(worktree .. "/renamed.txt"))
local buf = api.nvim_get_current_buf()

api.nvim_win_set_cursor(new, { 2, 0 })
vim.cmd("Revisa comment")
local float = H.float()
api.nvim_buf_set_lines(float, 0, -1, false, { "Comment on the new name" })
vim.cmd("write")
H.display(buf, 2, 2, { "Comment on the new name" })
