-- Comment lines wrap to the text width the window has once their signs
-- are drawn: 'signcolumn' auto adds a gutter the first draw creates.
vim.o.columns, vim.o.lines = 80, 30
vim.o.signcolumn = "auto"
local buf, win = H.open()
local room = vim.api.nvim_win_get_width(win)
    - vim.fn.getwininfo(win)[1].textoff
H.equal(78, room)

local widest = 0
for _, block in ipairs(H.virtual(buf)) do
    for _, line in ipairs(block.lines) do
        widest = math.max(widest, vim.fn.strdisplaywidth(line[1][1]))
    end
end
H.equal(true, widest <= room)

H.equal(true, H.row(H.screen(), "XY" .. string.rep("z", 5)) ~= nil)
