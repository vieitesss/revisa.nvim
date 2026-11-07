local api = vim.api
H.open()
require("revisa").setup({})
vim.cmd("runtime plugin/revisa.lua")
H.equal(2, vim.fn.exists(":Revisa"))
for _, mode in ipairs({ "n", "x" }) do
    H.equal("<Plug>(revisa-comment)",
        vim.fn.maparg("<Plug>(revisa-comment)", mode, false, true).lhs)
end

api.nvim_set_hl(0, "RevisaSign", { link = "Error" })
api.nvim_set_hl(0, "RevisaComment", { link = "String" })
vim.g.loaded_revisa = nil
vim.cmd("runtime plugin/revisa.lua")
H.equal("Error", api.nvim_get_hl(0, { name = "RevisaSign" }).link)
H.equal("String", api.nvim_get_hl(0, { name = "RevisaComment" }).link)

local messages = H.notifications()
vim.cmd("Revisa unknown")
H.error(messages, "use comment or refresh")
vim.opt.rtp:prepend(vim.env.REVISA_HELP_ROOT)
vim.cmd.helptags(vim.fn.fnameescape(vim.env.REVISA_HELP_ROOT .. "/doc"))
vim.cmd("help :Revisa-comment")
H.equal("revisa.txt", vim.fs.basename(api.nvim_buf_get_name(0)))
