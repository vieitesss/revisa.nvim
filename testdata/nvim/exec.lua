-- A failed command reports its stderr, else "<what> failed (exit N)",
-- whether the failed process is the CLI, rex or recado.
local api = vim.api
local buf, win = H.open()
vim.env.PATH = vim.env.REVISA_STUBS .. ":" .. vim.env.PATH
vim.env.REX_SESSION = "test"
local messages = H.notifications()

local cases = {
    { process = "revisa", command = "refresh",
        message = "revisa failed (exit 6)" },
    { process = "rex", command = "send", message = "rex failed (exit 4)" },
    { process = "recado", command = "send",
        message = "recado agents failed (exit 5)" },
}
for _, case in ipairs(cases) do
    vim.env.REVISA_PROCESS = case.process
    if case.command == "send" then
        require("revisa").setup({ notify = case.process })
    end
    local before = #messages
    vim.cmd("Revisa " .. case.command)
    H.equal(before + 1, #messages)
    local expected = vim.env.REVISA_CASE == "stderr" and "oops"
        or case.message
    H.equal(expected, messages[#messages].message)
    H.equal(vim.log.levels.ERROR, messages[#messages].level)
    H.equal(buf, api.nvim_get_current_buf())
    H.equal(win, api.nvim_get_current_win())
    H.equal(1, #api.nvim_list_wins())
end
