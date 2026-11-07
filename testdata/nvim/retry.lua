local messages = H.notifications()
local buf, win = H.open()
vim.cmd("2Revisa comment")
local editor, float = H.reserved(buf, win, 2)
vim.api.nvim_buf_set_lines(editor, 0, -1, false, { "Retry this body" })

-- Unsaved edits made while the draft is open must also be refused.
vim.api.nvim_buf_set_lines(buf, 0, 1, false, { "unsaved" })
vim.cmd("write")
H.error(messages, "save the buffer before commenting")
H.reserved(buf, win, 2)
H.equal({ "Retry this body" },
    vim.api.nvim_buf_get_lines(editor, 0, -1, false))
vim.api.nvim_buf_set_lines(buf, 0, 1, false, { "local a = 1" })
vim.bo[buf].modified = false

local function checkout(revision)
    local result = vim.system({
        "git", "-C", vim.env.REVISA_WORKTREE, "checkout", "-q", revision,
    }, { text = true }):wait()
    H.equal(0, result.code)
end
checkout("--detach")
vim.cmd("write")
H.error(messages, "add: HEAD is detached, pass --branch")
H.reserved(buf, win, 2)
H.equal(float, vim.api.nvim_get_current_win())
H.equal({ "Retry this body" },
    vim.api.nvim_buf_get_lines(editor, 0, -1, false))
H.equal("[]", H.cli({ "list", "--json", "--branch", "feat/x" }))
checkout("feat/x")
vim.cmd("write")
assert(not H.padding(buf), "padding remains after retry succeeds")
H.display(buf, 2, 2, { "Retry this body" })
