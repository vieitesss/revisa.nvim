local api = vim.api
local case = vim.env.REVISA_CASE

local function lines(buf)
    return api.nvim_buf_get_lines(buf, 0, -1, false)
end

local function find_line(buf, text)
    for i, line in ipairs(lines(buf)) do
        if line == text then
            return i
        end
    end
    error("no line " .. text .. " in " .. vim.inspect(lines(buf)))
end

local function expected()
    return {
        "worktree " .. vim.env.REVISA_EXPECT,
        "branch feat/x",
        "base " .. vim.env.REVISA_BASE .. " (main)",
        "",
        "? notes.txt",
        "M src/a.lua",
        "D src/gone.go",
        "A src/new.go",
        "R src/moved.go -> src/renamed.go",
    }
end

-- check_buffer asserts the changes buffer of the current window.
local function check_buffer()
    local buf = api.nvim_get_current_buf()
    H.equal("revisa://changes/feat/x", api.nvim_buf_get_name(buf))
    H.equal("nofile", vim.bo[buf].buftype)
    H.equal(false, vim.bo[buf].modifiable)
    H.equal(expected(), lines(buf))
    return buf
end

-- check_list reviews one worktree and checks the changes buffer.
local function check_list(request)
    local origin = api.nvim_get_current_win()
    require("revisa").review(request)
    assert(api.nvim_get_current_win() ~= origin, "list reused the window")
    return check_buffer()
end

if case == "list" then
    H.open()
    local origin = api.nvim_get_current_win()
    vim.cmd("Revisa review " .. vim.fn.fnameescape(vim.env.REVISA_TARGET))
    assert(api.nvim_get_current_win() ~= origin, "list reused the window")
    local list = check_buffer()
    local window = api.nvim_get_current_win()
    api.nvim_win_set_cursor(window, { 1, 0 })
    local messages = H.notifications()
    return function()
        H.keys("o<CR>")
        return function()
            H.equal({}, messages)
            H.equal(window, api.nvim_get_current_win())
            H.equal(list, api.nvim_get_current_buf())
            H.equal(1, #api.nvim_list_tabpages())
            vim.cmd.close()
            check_list("feat/x")
        end
    end
elseif case == "open" then
    local _, origin = H.open()
    local list = check_list(vim.env.REVISA_TARGET)
    api.nvim_win_set_cursor(0, { find_line(list, "M src/a.lua"), 0 })
    return function()
        H.keys("o")
        return function()
            H.equal(origin, api.nvim_get_current_win())
            H.equal(vim.env.REVISA_OPENED, api.nvim_buf_get_name(0))
            H.display(api.nvim_get_current_buf(), 2, 2, { "Handle nil" })
        end
    end
elseif case == "outside" then
    -- The working directory is in no git repository, so only the
    -- explicit worktree decides where the CLI runs.
    H.equal("", api.nvim_buf_get_name(0))
    check_list(vim.env.REVISA_TARGET)
elseif case == "startup" then
    -- The shell command leaves its worktree in the environment. One
    -- :Revisa review consumes it: the next one opens the picker instead
    -- of reopening that worktree, and no nested Neovim inherits it.
    H.open()
    local origin = api.nvim_get_current_win()
    vim.cmd("Revisa review")
    assert(api.nvim_get_current_win() ~= origin, "list reused the window")
    check_buffer()
    H.equal(nil, vim.env.REVISA_REVIEW_WORKTREE)
    vim.cmd.close()
    H.equal(origin, api.nvim_get_current_win())
    local offered
    vim.ui.select = function(items, options, on_choice)
        offered = items
        on_choice(nil, nil)
    end
    vim.cmd("Revisa review")
    assert(offered, "second :Revisa review opened no picker")
    H.equal(origin, api.nvim_get_current_win())
elseif case == "deleted" then
    local messages = H.notifications()
    H.open()
    local list = check_list(vim.env.REVISA_TARGET)
    local window = api.nvim_get_current_win()
    api.nvim_win_set_cursor(0, { find_line(list, "D src/gone.go"), 0 })
    return function()
        H.keys("o")
        return function()
            H.error(messages, "src/gone.go is deleted in")
            H.equal(window, api.nvim_get_current_win())
            H.equal(list, api.nvim_get_current_buf())
            H.equal(2, #api.nvim_list_wins())
        end
    end
elseif case == "reload" then
    local _, origin = H.open()
    local list = check_list(vim.env.REVISA_TARGET)
    local file = assert(io.open(vim.env.REVISA_NEW_FILE, "w"))
    file:write("added after the list\n")
    file:close()
    return function()
        H.keys("R")
        return function()
            local after = lines(list)
            H.equal("revisa://changes/feat/x", api.nvim_buf_get_name(list))
            H.equal(false, vim.bo[list].modifiable)
            H.equal("? late.txt", after[5])
            H.equal(10, #after)
            api.nvim_win_set_cursor(0, { 5, 0 })
            H.keys("o")
            return function()
                H.equal(origin, api.nvim_get_current_win())
                H.equal(vim.uv.fs_realpath(vim.env.REVISA_NEW_FILE),
                    api.nvim_buf_get_name(0))
                H.equal({ "added after the list" }, lines(0))
            end
        end
    end
elseif case == "close" then
    local _, origin = H.open()
    check_list(vim.env.REVISA_TARGET)
    return function()
        H.keys("q")
        return function()
            H.equal(origin, api.nvim_get_current_win())
            H.equal(1, #api.nvim_list_wins())
        end
    end
elseif case == "picker" or case == "picker-cancel" then
    H.open()
    local items, prompt
    vim.ui.select = function(list, options, on_choice)
        items = list
        prompt = options.prompt
        if case == "picker-cancel" then
            on_choice(nil, nil)
            return
        end
        for index, item in ipairs(list) do
            if item:find("feat/x  ", 1, true) then
                on_choice(item, index)
                return
            end
        end
        error("feat/x missing from " .. vim.inspect(list))
    end
    require("revisa").review()
    table.sort(items)
    H.equal({
        "feat/x  " .. vim.env.REVISA_EXPECT,
        "main  " .. vim.env.REVISA_MAIN,
    }, items)
    assert(prompt:find("worktree", 1, true), prompt)
    if case == "picker-cancel" then
        H.equal(1, #api.nvim_list_wins())
        return
    end
    H.equal("revisa://changes/feat/x", api.nvim_buf_get_name(0))
    H.equal(expected(), lines(0))
else
    error("unknown case " .. case)
end
