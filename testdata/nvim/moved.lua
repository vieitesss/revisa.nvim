local buf = H.open()
vim.cmd("Revisa refresh")

-- Two lines were inserted above the comment, so its sign moved with its
-- anchor text and no marker stands in for it.
H.display(buf, 3, 3, { "Kept" })
H.equal(nil, H.text(buf))

-- Nothing is rendered in the end block, and the summary is not there.
vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
H.equal(nil, H.row(rows, "outdated ("))
H.equal(nil, H.row(rows, "revisa:"))
