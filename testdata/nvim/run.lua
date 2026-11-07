dofile(vim.env.REVISA_HELPERS)

local function run(test)
    local ok, next_step = xpcall(test, debug.traceback)
    if not ok then
        io.stderr:write(next_step .. "\n")
        vim.cmd("cquit 1")
    elseif type(next_step) == "function" then
        -- Return to the input loop before checking queued key presses.
        vim.defer_fn(function()
            run(next_step)
        end, 100)
    else
        vim.cmd("qa!")
    end
end

run(function()
    return dofile(vim.env.REVISA_SCRIPT)
end)
