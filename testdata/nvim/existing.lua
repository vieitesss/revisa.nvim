local buf, win = H.open()
local last = tonumber(vim.env.REVISA_LINE)
H.display(buf, last, last, { "Existing body", "Second line" })
vim.cmd(last .. "Revisa comment")
local _, float = H.reserved(buf, win, last)
local position = vim.fn.win_screenpos(float)
local row = position[1] + vim.api.nvim_win_get_height(float)
local col = vim.fn.win_screenpos(win)[2] + vim.fn.getwininfo(win)[1].textoff
for offset, text in ipairs({ "Existing body", "Second line" }) do
    local visible = ""
    for column = col, col + #text - 1 do
        visible = visible .. vim.fn.screenstring(row + offset - 1, column)
    end
    H.equal(text, visible)
end
H.keys("q")
assert(not H.padding(buf), "padding remains after cancel")
H.display(buf, last, last, { "Existing body", "Second line" })
