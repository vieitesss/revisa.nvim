local buf = H.open()
vim.cmd("Revisa refresh")

H.display(buf, 3, 3, { "Kept" })
H.equal(nil, H.text(buf))

vim.o.lines = 20
vim.o.columns = 80
local rows = H.screen()
H.equal(nil, H.row(rows, "outdated ("))
H.equal(nil, H.row(rows, "revisa:"))
