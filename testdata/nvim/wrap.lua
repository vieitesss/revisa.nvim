vim.o.columns, vim.o.lines = 120, 30
local buf, win = H.open()

local function body()
    for _, block in ipairs(H.virtual(buf)) do
        local text = {}
        for _, line in ipairs(block.lines) do
            text[#text + 1] = line[1][1]
        end
        return text
    end
end

local function widest(lines)
    local cells = 0
    for _, line in ipairs(lines) do
        cells = math.max(cells, vim.fn.strdisplaywidth(line))
    end
    return cells
end

local wide = body()
H.equal(true, widest(wide) <= 80)
H.equal(true, widest(wide) > 60)
local rows = H.screen()
H.equal(true, H.row(rows, wide[2]) ~= nil)

vim.cmd("rightbelow vnew")
vim.api.nvim_set_current_win(win)
vim.cmd("vertical resize 40")
return function()
    local textoff = vim.fn.getwininfo(win)[1].textoff
    local narrow = body()
    H.equal(true, widest(narrow) <= 40 - textoff)
    H.equal(true, #narrow > #wide)
    vim.cmd("vertical resize 10")
    return function()
        H.equal(true, widest(body()) <= 20)
        H.equal(true, widest(body()) > 10)
    end
end
