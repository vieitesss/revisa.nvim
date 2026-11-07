if vim.g.loaded_revisa then
    return
end
vim.g.loaded_revisa = true

local api = vim.api
api.nvim_set_hl(0, "RevisaSign", {
    link = "DiagnosticSignInfo", default = true,
})
api.nvim_set_hl(0, "RevisaComment", { link = "Comment", default = true })
api.nvim_set_hl(0, "RevisaResolved", {
    link = "DiagnosticSignOk", default = true,
})
api.nvim_set_hl(0, "RevisaReply", { link = "DiagnosticOk", default = true })

api.nvim_create_user_command("Revisa", function(command)
    if command.args == "comment" then
        require("revisa").comment(command.line1, command.line2)
    elseif command.args == "refresh" then
        require("revisa").refresh()
    elseif command.args == "resolved" then
        require("revisa").toggle_resolved()
    else
        require("revisa.cli").error(
            "revisa: use comment, refresh or resolved")
    end
end, {
    nargs = 1,
    range = true,
    complete = function()
        return { "comment", "refresh", "resolved" }
    end,
})

vim.keymap.set("n", "<Plug>(revisa-comment)", function()
    local line = api.nvim_win_get_cursor(0)[1]
    require("revisa").comment(line, line)
end)
vim.keymap.set("x", "<Plug>(revisa-comment)",
    ":<C-u>'<,'>Revisa comment<CR>")

api.nvim_create_autocmd("BufReadPost", {
    group = api.nvim_create_augroup("revisa", { clear = true }),
    callback = function(event)
        if require("revisa.buffer").file(event.buf) then
            require("revisa").refresh(event.buf)
        end
    end,
})
