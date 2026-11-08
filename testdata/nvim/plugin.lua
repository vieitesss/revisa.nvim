local api = vim.api
H.open()
require("revisa").setup({})
vim.cmd("runtime plugin/revisa.lua")
H.equal(2, vim.fn.exists(":Revisa"))
H.equal("DiagnosticWarn",
    api.nvim_get_hl(0, { name = "RevisaOutdated" }).link)
H.equal("DiagnosticError",
    api.nvim_get_hl(0, { name = "RevisaUnknown" }).link)
H.equal("Comment", api.nvim_get_hl(0, { name = "RevisaWas" }).link)
for _, mode in ipairs({ "n", "x" }) do
    H.equal("<Plug>(revisa-comment)",
        vim.fn.maparg("<Plug>(revisa-comment)", mode, false, true).lhs)
end

api.nvim_set_hl(0, "RevisaSign", { link = "Error" })
api.nvim_set_hl(0, "RevisaComment", { link = "String" })
api.nvim_set_hl(0, "RevisaResolved", { link = "WarningMsg" })
api.nvim_set_hl(0, "RevisaReply", { link = "Title" })
api.nvim_set_hl(0, "RevisaOutdated", { link = "WarningMsg" })
api.nvim_set_hl(0, "RevisaUnknown", { link = "ErrorMsg" })
api.nvim_set_hl(0, "RevisaWas", { link = "String" })
vim.g.loaded_revisa = nil
vim.cmd("runtime plugin/revisa.lua")
H.equal("Error", api.nvim_get_hl(0, { name = "RevisaSign" }).link)
H.equal("String", api.nvim_get_hl(0, { name = "RevisaComment" }).link)
H.equal("WarningMsg",
    api.nvim_get_hl(0, { name = "RevisaResolved" }).link)
H.equal("Title", api.nvim_get_hl(0, { name = "RevisaReply" }).link)
H.equal("WarningMsg",
    api.nvim_get_hl(0, { name = "RevisaOutdated" }).link)
H.equal("ErrorMsg",
    api.nvim_get_hl(0, { name = "RevisaUnknown" }).link)
H.equal("String", api.nvim_get_hl(0, { name = "RevisaWas" }).link)

local messages = H.notifications()
vim.cmd("Revisa unknown")
H.error(messages, "use comment, refresh, resolved, review, send or beside")
vim.cmd("Revisa")
H.error(messages, "use comment, refresh, resolved, review, send or beside")
vim.cmd("Revisa refresh extra")
H.error(messages, "refresh takes no argument")
vim.opt.rtp:prepend(vim.env.REVISA_HELP_ROOT)
vim.cmd.helptags(vim.fn.fnameescape(vim.env.REVISA_HELP_ROOT .. "/doc"))
vim.cmd("help :Revisa-comment")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help :Revisa-resolved")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help :Revisa-send")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help RevisaOutdated")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help RevisaUnknown")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help RevisaWas")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
vim.cmd("help :Revisa-beside")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
