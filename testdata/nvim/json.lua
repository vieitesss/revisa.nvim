-- Every caller that decodes CLI output comes through the one seam, which
-- reports a bad payload the same way, once. The run is stubbed, so no
-- binary or project is needed.
local cli = require("revisa.cli")
local messages = H.notifications()
local worktree = vim.env.REVISA_WORKTREE
cli.run = function()
    return "{"
end
local shared = cli.json
local calls = 0
cli.json = function(context, args)
    calls = calls + 1
    return shared(context, args)
end

local buf = vim.api.nvim_get_current_buf()
vim.b[buf].revisa_target = {
    name = vim.api.nvim_buf_get_name(buf),
    worktree = worktree,
    path = "a.lua",
    side = "new",
    branch = "main",
}
require("revisa").refresh(buf)
require("revisa.review").open(worktree)
require("revisa.notify.recado").available = function()
    return true
end
require("revisa.notify").setup({ notify = "recado" })
require("revisa.notify").send(worktree)

H.equal(3, calls)
H.equal(3, #messages)
for _, message in ipairs(messages) do
    H.equal(vim.log.levels.ERROR, message.level)
    assert(message.message:find("revisa: invalid JSON from CLI: ", 1, true),
        message.message)
end
H.equal(nil, cli.json({ cwd = worktree }, { "list", "--json" }))
H.equal(4, #messages)
