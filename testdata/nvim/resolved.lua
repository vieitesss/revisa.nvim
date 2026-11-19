local api = vim.api
local buf = H.open()

local id = H.cli({
    "add", "--path", "src/a.lua", "--lines", "2", "--body", "Handle nil",
})
vim.cmd("Revisa refresh")
H.display(buf, 2, 2, { "Handle nil" })
H.cli({ "resolve", id, "--reply", "Fixed it\nSecond line" })
vim.cmd("Revisa refresh")
H.equal({}, H.marks(buf))

vim.cmd.edit(vim.fn.fnameescape(vim.env.REVISA_OTHER))
local other = api.nvim_get_current_buf()
local otherID = H.cli({
    "add", "--path", "src/b.lua", "--lines", "1", "--body", "Other body",
})
H.cli({ "resolve", otherID, "--reply", "Done" })
vim.cmd("Revisa refresh")
H.equal({}, H.marks(other))

vim.cmd.buffer(buf)
vim.cmd("Revisa resolved")
H.resolved(buf, 2, 2, {
    { "Handle nil", "RevisaComment" },
    { "reply: Fixed it", "RevisaReply" },
    { "Second line", "RevisaReply" },
})
H.resolved(other, 1, 1, {
    { "Other body", "RevisaComment" },
    { "reply: Done", "RevisaReply" },
})
H.equal("DiagnosticSignOk",
    api.nvim_get_hl(0, { name = "RevisaResolved" }).link)
H.equal("DiagnosticOk", api.nvim_get_hl(0, { name = "RevisaReply" }).link)

vim.cmd("Revisa refresh")
H.resolved(buf, 2, 2, {
    { "Handle nil", "RevisaComment" },
    { "reply: Fixed it", "RevisaReply" },
    { "Second line", "RevisaReply" },
})

require("revisa").toggle_resolved()
H.equal({}, H.marks(buf))
H.equal({}, H.marks(other))
