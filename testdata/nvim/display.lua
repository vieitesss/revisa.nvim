local buf = H.open()
H.display(buf, 2, 4, { "Handle nil", "All three lines." })
H.equal("DiagnosticSignInfo",
    vim.api.nvim_get_hl(0, { name = "RevisaSign" }).link)
H.equal("Comment", vim.api.nvim_get_hl(0, { name = "RevisaComment" }).link)
