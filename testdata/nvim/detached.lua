local api = vim.api
local target = require("revisa.target")

-- Two detached worktrees have no branch to name their changes buffer, so
-- its name must come from the head and the worktree. Otherwise the second
-- review deletes the first one's buffer.

local function nameOf(path)
    require("revisa").review(path)
    local buf = api.nvim_get_current_buf()
    H.equal("changes", target.of(buf).kind)
    H.equal("", target.of(buf).branch)
    return buf, api.nvim_buf_get_name(buf)
end

local first, firstName = nameOf(vim.env.REVISA_TARGET_A)
local second, secondName = nameOf(vim.env.REVISA_TARGET_B)

assert(first ~= second, "the second review reused the first buffer")
assert(api.nvim_buf_is_valid(first), "the second review deleted the first")
assert(firstName ~= secondName, "both detached worktrees share a name")

local names = { firstName, secondName }
table.sort(names)
local want = vim.json.decode(vim.env.REVISA_NAMES)
table.sort(want)
H.equal(want, names)
