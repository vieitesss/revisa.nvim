local api = vim.api
H = {}

function H.equal(want, got)
    assert(vim.deep_equal(want, got),
        "want " .. vim.inspect(want) .. ", got " .. vim.inspect(got))
end

function H.notifications()
    local messages = {}
    vim.notify = function(message, level)
        messages[#messages + 1] = { message = message, level = level }
    end
    return messages
end

function H.error(messages, text)
    assert(#messages > 0, "no error notification")
    local message = messages[#messages]
    H.equal(vim.log.levels.ERROR, message.level)
    assert(message.message:find(text, 1, true), message.message)
end

function H.keys(keys)
    api.nvim_feedkeys(api.nvim_replace_termcodes(keys, true, false, true),
        "mxt", false)
end

function H.cli(args)
    local command = { "revisa" }
    vim.list_extend(command, args)
    local result = vim.system(command, {
        cwd = vim.env.REVISA_WORKTREE,
        text = true,
    }):wait()
    H.equal(0, result.code)
    return vim.trim(result.stdout)
end

function H.open()
    vim.cmd.edit(vim.fn.fnameescape(vim.env.REVISA_FILE))
    return api.nvim_get_current_buf(), api.nvim_get_current_win()
end

function H.marks(buf)
    return api.nvim_buf_get_extmarks(buf, -1, 0, -1, { details = true })
end

function H.display(buf, first, last, body)
    local signs, bodies = {}, {}
    for _, mark in ipairs(H.marks(buf)) do
        local details = mark[4]
        if details.sign_text then
            H.equal(">", vim.trim(details.sign_text))
            H.equal("RevisaSign", details.sign_hl_group)
            signs[#signs + 1] = mark[2] + 1
        end
        if details.virt_lines then
            local text = {}
            for _, line in ipairs(details.virt_lines) do
                H.equal("RevisaComment", line[1][2])
                text[#text + 1] = line[1][1]
            end
            local last = mark[2] + (details.virt_lines_above and 0 or 1)
            bodies[#bodies + 1] = { last, text }
        end
    end
    table.sort(signs)
    local expected = {}
    for line = first, last do
        expected[#expected + 1] = line
    end
    H.equal(expected, signs)
    H.equal({ { last, body } }, bodies)
end

function H.resolved(buf, first, last, lines)
    local signs, bodies = {}, {}
    for _, mark in ipairs(H.marks(buf)) do
        local details = mark[4]
        if details.sign_text then
            H.equal("✓", vim.trim(details.sign_text))
            H.equal("RevisaResolved", details.sign_hl_group)
            signs[#signs + 1] = mark[2] + 1
        end
        if details.virt_lines then
            local text = {}
            for _, line in ipairs(details.virt_lines) do
                text[#text + 1] = { line[1][1], line[1][2] }
            end
            local at = mark[2] + (details.virt_lines_above and 0 or 1)
            bodies[#bodies + 1] = { at, text }
        end
    end
    table.sort(signs)
    local expected = {}
    for line = first, last do
        expected[#expected + 1] = line
    end
    H.equal(expected, signs)
    H.equal({ { last, lines } }, bodies)
end

function H.padding(buf)
    for _, mark in ipairs(H.marks(buf)) do
        local lines = mark[4].virt_lines
        if lines and #lines > 0 and lines[1][1][1] == "" then
            return mark
        end
    end
end

function H.reserved(buf, win, last)
    local editor, float = H.float()
    local padding = H.padding(buf)
    assert(padding, "no reserved virtual lines")
    H.equal(last - 1, padding[2])
    H.equal(api.nvim_win_get_height(float), #padding[4].virt_lines)
    vim.cmd.redraw()
    local line = api.nvim_buf_get_lines(buf, last - 1, last, false)[1]
    local selected = vim.fn.screenpos(win, last, math.max(1, #line))
    assert(selected.row > 0, "last selected line is not visible")
    local position = vim.fn.win_screenpos(float)
    H.equal(selected.row + 1, position[1])
    local source_position = vim.fn.win_screenpos(win)
    assert(position[2]
        == source_position[2] + vim.fn.getwininfo(win)[1].textoff,
        "wrong float column: " .. vim.inspect({ selected, position,
            source_position, api.nvim_win_get_config(float),
            api.nvim_win_get_position(float) }))
    assert(position[1] + api.nvim_win_get_height(float)
        <= source_position[1] + api.nvim_win_get_height(win),
        "float extends outside the file window: "
            .. vim.inspect({ selected, position, source_position,
                api.nvim_win_get_height(win),
                vim.api.nvim_win_call(win, vim.fn.winsaveview) }))
    if last < api.nvim_buf_line_count(buf) then
        local next_line = vim.fn.screenpos(win, last + 1, 1)
        assert(next_line.row >= position[1] + api.nvim_win_get_height(float),
            "float covers the next file line")
    end
    return editor, float
end

function H.float()
    for _, win in ipairs(api.nvim_list_wins()) do
        if api.nvim_win_get_config(win).relative ~= "" then
            return api.nvim_win_get_buf(win), win
        end
    end
    error("comment float not found")
end
