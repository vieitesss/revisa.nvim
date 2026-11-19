local messages = H.notifications()

local function notified(text)
    for _, message in ipairs(messages) do
        if message.message:find(text, 1, true) then
            return true
        end
    end
    return false
end

local function finished()
    return notified("CLI built") or notified("failed")
end

local case = vim.env.REVISA_CASE
if case == "built" then
    assert(vim.wait(10000, finished), "the build did not finish")
    H.equal({ message = "revisa: CLI built", level = vim.log.levels.INFO },
        messages[#messages])
elseif case == "failed" then
    assert(vim.wait(10000, finished), "the build did not finish")
    H.error(messages, "revisa: building the CLI failed: boom")
    H.error(messages, "vim.g.revisa_auto_build = false")
    vim.cmd("Revisa review")
    H.error(messages, "revisa not found")
elseif case == "immediate" then
    vim.cmd("Revisa review")
    H.error(messages, "revisa: the CLI is still building")
    assert(vim.wait(10000, finished), "the build did not finish")
elseif case == "building" then
    assert(vim.wait(10000, function()
        return notified("revisa: building the CLI")
    end), "the build did not start")
    vim.cmd("Revisa review")
    H.error(messages, "revisa: the CLI is still building")
    assert(vim.wait(10000, finished), "the build did not finish")
elseif case == "idle" then
    vim.wait(500)
    H.equal({}, messages)
else
    error("unknown case " .. tostring(case))
end
