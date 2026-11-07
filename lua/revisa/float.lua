local M = {}
local api = vim.api
local padding = api.nvim_create_namespace("revisa.padding")

function M.open(buf, last, save)
    local source = api.nvim_get_current_win()
    local text = api.nvim_buf_get_lines(buf, last - 1, last, false)[1]
    local column = math.max(0, #text - 1)
    api.nvim_win_set_cursor(source, { last, column })
    api.nvim_win_call(source, function()
        vim.cmd("normal! zv")
    end)
    local height = math.min(4, api.nvim_win_get_height(source) - 1)
    height = math.max(1, height)
    local lines = {}
    for _ = 1, height do
        lines[#lines + 1] = { { "", "Normal" } }
    end
    local mark = api.nvim_buf_set_extmark(buf, padding, last - 1, 0, {
        virt_lines = lines,
    })
    vim.cmd.redraw()
    local position = vim.fn.screenpos(source, last, math.max(1, #text))
    local top = vim.fn.win_screenpos(source)
    if position.row == 0
        or position.row + height >= top[1] + api.nvim_win_get_height(source)
    then
        api.nvim_win_call(source, function()
            vim.cmd("normal! zt")
        end)
        vim.cmd.redraw()
        position = vim.fn.screenpos(source, last, math.max(1, #text))
    end
    local offset = vim.fn.getwininfo(source)[1].textoff
    if position.row + height >= top[1] + api.nvim_win_get_height(source) then
        api.nvim_win_call(source, function()
            local view = vim.fn.winsaveview()
            local width = math.max(1, api.nvim_win_get_width(source) - offset)
            view.skipcol = view.skipcol + height * width
            vim.fn.winrestview(view)
        end)
        vim.cmd.redraw()
        position = vim.fn.screenpos(source, last, math.max(1, #text))
    end
    local editor = api.nvim_create_buf(false, true)
    vim.bo[editor].buftype = "acwrite"
    vim.bo[editor].bufhidden = "wipe"
    vim.bo[editor].swapfile = false
    api.nvim_buf_set_name(editor, "revisa-comment://" .. editor)
    local win = api.nvim_open_win(editor, true, {
        relative = "editor",
        row = position.row,
        col = top[2] - 1 + offset,
        width = math.max(1,
            math.min(60, api.nvim_win_get_width(source) - offset)),
        height = height,
        style = "minimal",
        border = "none",
    })
    vim.wo[win].diff = false
    vim.wo[win].scrollbind = false
    vim.wo[win].cursorbind = false
    local scroll = api.nvim_create_autocmd("WinScrolled", {
        pattern = tostring(source),
        callback = function()
            if api.nvim_win_is_valid(win) then
                local anchor = vim.fn.screenpos(source, last, column + 1)
                local origin = vim.fn.win_screenpos(source)
                api.nvim_win_set_config(win, {
                    relative = "editor",
                    row = anchor.row,
                    col = origin[2] - 1
                        + vim.fn.getwininfo(source)[1].textoff,
                })
            end
        end,
    })
    local function cleanup()
        if scroll then
            api.nvim_del_autocmd(scroll)
            scroll = nil
        end
        if api.nvim_buf_is_valid(buf) then
            api.nvim_buf_del_extmark(buf, padding, mark)
        end
    end
    local function close()
        cleanup()
        if api.nvim_win_is_valid(win) then
            api.nvim_win_close(win, true)
        end
    end
    local function write()
        local body = table.concat(
            api.nvim_buf_get_lines(editor, 0, -1, false), "\n")
        if vim.trim(body) == "" or save(body) then
            close()
        end
    end
    api.nvim_create_autocmd("BufWipeout", {
        buffer = editor,
        once = true,
        callback = cleanup,
    })
    api.nvim_create_autocmd("BufWriteCmd", {
        buffer = editor,
        callback = write,
    })
    vim.keymap.set("n", "q", close, { buffer = editor })
    vim.keymap.set({ "n", "i" }, "<C-s>", write, { buffer = editor })
end

return M
