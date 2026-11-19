local api = vim.api
local messages = H.notifications()

local function save(text)
    vim.cmd("1Revisa comment")
    local editor = H.float()
    api.nvim_buf_set_lines(editor, 0, -1, false, { text })
    vim.cmd("write")
end

H.open()
save("Linked file")
vim.cmd.edit(vim.fn.fnameescape(vim.env.REVISA_MAIN_FILE))
save("Main file")

vim.cmd.enew()
api.nvim_buf_set_name(0, vim.env.REVISA_FILE .. ".missing")
vim.cmd("Revisa comment")
H.error(messages, "revisa: comments require a file on disk")
vim.cmd.enew()
api.nvim_buf_set_name(0, "tool://old-side/src/a.lua")
vim.cmd("Revisa comment")
H.error(messages, "revisa: cannot comment in a tool:// buffer")

require("revisa").review(vim.env.REVISA_TARGET)
local list = api.nvim_get_current_buf()
vim.cmd("Revisa comment")
H.error(messages, "revisa: comments require a real file buffer")
api.nvim_win_set_cursor(0, { 2, 0 })
H.keys("<CR>")
return function()
    H.equal(list, api.nvim_get_current_buf())
    for line, text in ipairs(api.nvim_buf_get_lines(list, 0, -1, false)) do
        if text == "D src/gone.go" then
            api.nvim_win_set_cursor(0, { line, 0 })
            H.keys("<CR>")
            return function()
                H.equal({ "package gone" },
                    api.nvim_buf_get_lines(0, 0, -1, false))
                save("Deleted old file")
                for _, win in ipairs(api.nvim_tabpage_list_wins(0)) do
                    local name = api.nvim_buf_get_name(
                        api.nvim_win_get_buf(win))
                    if name == "revisa://deleted/src/gone.go" then
                        api.nvim_set_current_win(win)
                        save("Must not store a deleted new side")
                        H.error(messages, "src/gone.go not found")
                        vim.cmd("normal q")
                        return
                    end
                end
                error("no deleted new side")
            end
        end
    end
    error("deleted entry not found")
end
