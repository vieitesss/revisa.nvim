local api = vim.api
local case = vim.env.REVISA_CASE

local function lines(buf)
    return api.nvim_buf_get_lines(buf, 0, -1, false)
end

local function env(name)
    return vim.json.decode(vim.env[name])
end

local function find(buf, text)
    for i, line in ipairs(lines(buf)) do
        if line == text then
            return i
        end
    end
    error("no line " .. text .. " in " .. vim.inspect(lines(buf)))
end

-- review opens the changes list and puts the cursor on the entry.
local function review()
    require("revisa").review(vim.env.REVISA_TARGET)
    local buf = api.nvim_get_current_buf()
    api.nvim_win_set_cursor(0, { find(buf, vim.env.REVISA_ENTRY), 0 })
    return buf
end

-- sides returns the old and new windows of the open diff tab.
local function sides()
    local old, new
    for _, win in ipairs(api.nvim_tabpage_list_wins(0)) do
        local name = api.nvim_buf_get_name(api.nvim_win_get_buf(win))
        if name:find("^revisa://deleted/") then
            new = win
        elseif name:find("^revisa://") then
            old = win
        else
            new = win
        end
    end
    assert(old and new, "diff sides not found")
    return old, new
end

local function check_old(win)
    local buf = api.nvim_win_get_buf(win)
    H.equal(vim.env.REVISA_OLD_NAME, api.nvim_buf_get_name(buf))
    H.equal("nofile", vim.bo[buf].buftype)
    H.equal(false, vim.bo[buf].modifiable)
    H.equal(false, vim.bo[buf].swapfile)
    H.equal("wipe", vim.bo[buf].bufhidden)
    local filetype = vim.env.REVISA_FILE_TYPE or ""
    if filetype ~= "" then
        H.equal(filetype, vim.bo[buf].filetype)
    end
    H.equal(env("REVISA_OLD_LINES"), lines(buf))
    local target = require("revisa.target").of(buf)
    H.equal({
        kind = "file",
        worktree = vim.env.REVISA_WORKTREE,
        cwd = vim.env.REVISA_WORKTREE,
        path = vim.env.REVISA_OLD_PATH,
        side = "old",
        commit = vim.env.REVISA_COMMIT,
        branch = "feat/x",
    }, target)
end

local function check_new(win)
    local buf = api.nvim_win_get_buf(win)
    H.equal(vim.env.REVISA_NEW_NAME, api.nvim_buf_get_name(buf))
    H.equal(env("REVISA_NEW_LINES"), lines(buf))
    H.equal({
        kind = "file",
        worktree = vim.env.REVISA_WORKTREE,
        cwd = vim.env.REVISA_WORKTREE,
        path = vim.env.REVISA_PATH,
        side = "new",
        branch = "feat/x",
    }, require("revisa.target").of(buf))
    if vim.env.REVISA_DELETED == "1" then
        H.equal("nofile", vim.bo[buf].buftype)
        H.equal(false, vim.bo[buf].modifiable)
        return
    end
    H.equal("", vim.bo[buf].buftype)
    H.equal(true, vim.bo[buf].modifiable)
end

-- check_diff asserts the whole diff tab: old left, new right, both in
-- diff mode, each side in the right buffer.
local function check_diff()
    H.equal(2, #api.nvim_list_tabpages())
    local old, new = sides()
    local layout = vim.fn.winlayout()
    H.equal("row", layout[1])
    H.equal(old, layout[2][1][2])
    H.equal(new, layout[2][2][2])
    H.equal(true, vim.wo[old].diff)
    H.equal(true, vim.wo[new].diff)
    H.equal(old, api.nvim_get_current_win())
    check_old(old)
    check_new(new)
    return old, new
end

-- check_old_comment asserts one old-side comment is drawn on line of
-- the old side, with body below it on screen, and never on the new side.
local function check_old_comment(old, new, line, body)
    local oldbuf = api.nvim_win_get_buf(old)
    local newbuf = api.nvim_win_get_buf(new)
    H.equal({ { line = line, text = ">", group = "RevisaSign" } },
        H.signs(oldbuf))
    H.equal({}, H.signs(newbuf))
    H.equal({
        {
            row = line,
            above = true,
            lines = { { { body, "RevisaComment" } } },
        },
    }, H.virtual(oldbuf))
    H.equal({}, H.virtual(newbuf))
    vim.o.lines = 20
    vim.o.columns = 100
    local rows = H.screen()
    local drawn = H.row(rows, body)
    assert(drawn, "old comment not rendered: " .. vim.inspect(rows))
    local position = vim.fn.screenpos(old, line, 1)
    assert(position.row > 0, "old line is not on screen")
    H.equal(position.row + 1, drawn)
end

-- comment writes body in the comment float of the current line.
local function comment(body)
    vim.cmd("Revisa comment")
    local editor = H.float()
    api.nvim_buf_set_lines(editor, 0, -1, false, { body })
    vim.cmd("write")
end

if case == "deleted" then
    review()
    return function()
        H.keys("<CR>")
        return function()
            local _, new = check_diff()
            local messages = H.notifications()
            api.nvim_set_current_win(new)
            vim.cmd("Revisa comment")
            local editor = H.float()
            api.nvim_buf_set_lines(editor, 0, -1, false, { "Gone" })
            vim.cmd("write")
            H.error(messages, "not found in")
        end
    end
elseif case == "deleted-outdated" then
    review()
    return function()
        H.keys("<CR>")
        return function()
            local _, new = check_diff()
            local buf = api.nvim_win_get_buf(new)
            H.equal({}, H.signs(buf))
            H.equal({
                {
                    row = 1,
                    above = true,
                    lines = {
                        { { "outdated (1)", "RevisaOutdated" } },
                        { { "Gone with the file", "RevisaComment" } },
                        { { "was: package gone", "RevisaWas" } },
                    },
                },
            }, H.virtual(buf))
            H.equal({ { "revisa: 1 outdated", "RevisaOutdated" } },
                H.text(buf))
            -- The block is rendered in the empty right window.
            vim.o.lines = 20
            vim.o.columns = 100
            local rows = H.screen()
            local heading = H.row(rows, "outdated (1)")
            assert(heading and heading > 1,
                "block not rendered: " .. vim.inspect(rows))
            assert(rows[heading + 1]:find("Gone with the file", 1, true),
                "body not rendered: " .. vim.inspect(rows))
            assert(rows[heading + 2]:find("was: package gone", 1, true),
                "was: not rendered: " .. vim.inspect(rows))
            local summary = H.row(rows, "revisa: 1 outdated")
            assert(summary and summary < heading,
                "summary not rendered above the block: "
                .. vim.inspect(rows))
        end
    end
elseif case == "editable" then
    review()
    return function()
        H.keys("<CR>")
        return function()
            local _, new = check_diff()
            api.nvim_set_current_win(new)
            api.nvim_buf_set_lines(api.nvim_win_get_buf(new), 0, -1, false,
                env("REVISA_WRITE_LINES"))
            vim.cmd.write()
            H.equal(false, vim.bo[api.nvim_win_get_buf(new)].modified)
        end
    end
elseif case == "close" then
    local list = review()
    return function()
        H.keys("<CR>")
        return function()
            local old = check_diff()
            api.nvim_set_current_win(old)
            H.keys("q")
            return function()
                H.equal(1, #api.nvim_list_tabpages())
                H.equal(list, api.nvim_get_current_buf())
                H.equal(2, #api.nvim_list_wins())
            end
        end
    end
elseif case == "old-comment" then
    review()
    return function()
        H.keys("<CR>")
        return function()
            local old, new = check_diff()
            check_old_comment(old, new, 2, "Old side body")
        end
    end
elseif case == "old-line" or case == "old-renamed" then
    local line = case == "old-line" and 2 or 3
    local body = case == "old-line" and "Old side body" or "Moved line"
    review()
    return function()
        H.keys("<CR>")
        return function()
            local old, new = check_diff()
            api.nvim_set_current_win(old)
            api.nvim_win_set_cursor(old, { line, 0 })
            comment(body)
            return function()
                check_old_comment(old, new, line, body)
            end
        end
    end
elseif case == "old-moved" then
    review()
    return function()
        H.keys("<CR>")
        return function()
            local old, new = check_diff()
            local oldbuf = api.nvim_win_get_buf(old)
            H.equal({}, H.signs(oldbuf))
            H.equal({}, H.virtual(oldbuf))
            H.equal(nil, H.text(oldbuf))
            H.equal({}, H.signs(api.nvim_win_get_buf(new)))
            H.equal(nil, H.row(H.screen(), "Old side body"))
        end
    end
elseif case == "binary" then
    local messages = H.notifications()
    local list = review()
    return function()
        H.keys("<CR>")
        return function()
            H.error(messages, vim.env.REVISA_BINARY)
            H.equal(1, #api.nvim_list_tabpages())
            H.equal(list, api.nvim_get_current_buf())
        end
    end
else
    review()
    return function()
        H.keys("<CR>")
        return function()
            check_diff()
        end
    end
end
