local build = require("revisa.build")
local messages = H.notifications()

local function finished()
    for _, message in ipairs(messages) do
        if message.message:find("CLI built", 1, true)
            or message.message:find("failed", 1, true) then
            return true
        end
    end
    return false
end

local case = vim.env.REVISA_CASE
if case == "built" then
    assert(vim.wait(10000, finished), "the build did not finish")
    H.equal({ message = "revisa: CLI built", level = vim.log.levels.INFO },
        messages[#messages])
    H.equal(false, build.building())
elseif case == "failed" then
    assert(vim.wait(10000, finished), "the build did not finish")
    H.error(messages, "revisa: building the CLI failed: boom")
    H.error(messages, "vim.g.revisa_auto_build = false")
    H.equal(false, build.building())
elseif case == "building" then
    assert(vim.wait(10000, build.building), "the build did not start")
    require("revisa.cli").run({ cwd = "." }, { "list" })
    H.error(messages, "revisa: the CLI is still building")
    assert(vim.wait(10000, finished), "the build did not finish")
elseif case == "idle" then
    vim.wait(500)
    H.equal(false, build.building())
    H.equal({}, messages)
end
