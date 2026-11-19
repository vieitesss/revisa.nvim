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
api.nvim_set_hl(0, "RevisaOutdated", {
    link = "DiagnosticWarn", default = true,
})
api.nvim_set_hl(0, "RevisaWas", { link = "Comment", default = true })

api.nvim_create_user_command("Revisa", function(command)
    local name, rest = command.args:match("^(%S+)%s*(.*)$")
    if not name then
        name, rest = "", ""
    end
    if name ~= "review" and name ~= "send" and rest ~= "" then
        require("revisa.cli").error(
            "revisa: " .. name .. " takes no argument")
        return
    end
    if name == "comment" then
        require("revisa").comment(command.line1, command.line2)
    elseif name == "edit" then
        require("revisa").edit()
    elseif name == "refresh" then
        require("revisa").refresh()
    elseif name == "resolved" then
        require("revisa").toggle_resolved()
    elseif name == "review" then
        require("revisa").review(rest)
    elseif name == "send" then
        require("revisa").send(rest)
    else
        require("revisa.cli").error(
            "revisa: use comment, edit, refresh, resolved, review or send")
    end
end, {
    nargs = "*",
    range = true,
    complete = function()
        return {
            "comment", "edit", "refresh", "resolved", "review", "send",
        }
    end,
})

vim.keymap.set("n", "<Plug>(revisa-comment)", function()
    local line = api.nvim_win_get_cursor(0)[1]
    require("revisa").comment(line, line)
end)
vim.keymap.set("x", "<Plug>(revisa-comment)",
    ":<C-u>'<,'>Revisa comment<CR>")

vim.keymap.set("n", "<Plug>(revisa-edit)", function()
    require("revisa").edit()
end)

local group = api.nvim_create_augroup("revisa", { clear = true })
api.nvim_create_autocmd("BufReadPost", {
    group = group,
    callback = function(event)
        if require("revisa.buffer").file(event.buf) then
            require("revisa").refresh(event.buf)
        end
    end,
})

api.nvim_create_autocmd({ "WinResized", "VimResized" }, {
    group = group,
    callback = function()
        require("revisa.display").redraw()
    end,
})

require("revisa.build").check()
