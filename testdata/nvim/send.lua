local api = vim.api
local case = vim.env.REVISA_CASE

local messages = H.notifications()
local offers = {}
local picks = {}
for pick in (vim.env.REVISA_PICK or "1"):gmatch("[^,]+") do
    picks[#picks + 1] = pick
end

vim.ui.select = function(items, options, on_choice)
    offers[#offers + 1] = { items = items, prompt = options.prompt }
    local pick = picks[#offers]
    if pick == nil or pick == "cancel" then
        on_choice(nil, nil)
        return
    end
    local index = tonumber(pick)
    on_choice(items[index], index)
end

local setting = vim.env.REVISA_SETTING or ""
if setting ~= "" then
    require("revisa").setup({ notify = setting })
end

local function send(argument)
    local command = "Revisa send"
    if argument and argument ~= "" then
        command = command .. " " .. vim.fn.fnameescape(argument)
    end
    vim.cmd(command)
end

local function check_offers(count)
    if count ~= #offers then
        error("want " .. count .. " pickers, got " .. #offers .. ": "
            .. vim.inspect(messages))
    end
    local panes = vim.env.REVISA_PANES or ""
    for _, offer in ipairs(offers) do
        assert(offer.prompt:find("send", 1, true), offer.prompt)
        assert(#offer.items > 0, "empty picker")
        if panes ~= "" then
            H.equal(vim.split(panes, "\n", { plain = true }), offer.items)
        end
        for id in (vim.env.REVISA_ABSENT or ""):gmatch("[^,]+") do
            for _, item in ipairs(offer.items) do
                assert(not item:find(id, 1, true),
                    id .. " was offered as " .. item)
            end
        end
    end
end

local function check_messages()
    local sent = vim.env.REVISA_SENT or ""
    if sent ~= "" then
        local last = messages[#messages]
        assert(last, "no notification after send")
        H.equal("revisa: sent to " .. sent, last.message)
        H.equal(vim.log.levels.INFO, last.level)
    end
    local expected = vim.env.REVISA_ERROR or ""
    if expected ~= "" then
        H.error(messages, expected)
    end
    assert(vim.env.RECADO_ADDR ~= "revisa", "RECADO_ADDR leaked into Neovim")
end

local function check_list()
    require("revisa").review(vim.env.REVISA_TARGET)
end

if case == "unavailable" then
    send()
    H.equal(0, #offers)
    check_messages()
elseif case == "invalid" then
    local first = messages[1]
    assert(first, "no error notification")
    H.equal(vim.log.levels.ERROR, first.level)
    assert(first.message:find(vim.env.REVISA_ERROR, 1, true), first.message)
    send()
    H.equal(0, #offers)
    assert(vim.env.RECADO_ADDR ~= "revisa", "RECADO_ADDR leaked into Neovim")
elseif case == "picker" or case == "argument" then
    H.open()
    send(vim.env.REVISA_TARGET)
    check_offers(tonumber(vim.env.REVISA_OFFERS))
    check_messages()
elseif case == "cancel" then
    H.open()
    send()
    check_offers(1)
    H.equal(0, #messages)
elseif case == "file" then
    H.open()
    send()
    check_offers(1)
    check_messages()
elseif case == "changes" then
    check_list()
    send()
    check_offers(1)
    check_messages()
elseif case == "diff" then
    check_list()
    local list = api.nvim_get_current_buf()
    local found = false
    for line, text in ipairs(api.nvim_buf_get_lines(list, 0, -1, false)) do
        if text:match("^%S+ src/a%.lua$") then
            api.nvim_win_set_cursor(0, { line, 0 })
            found = true
        end
    end
    assert(found, vim.inspect(api.nvim_buf_get_lines(list, 0, -1, false)))
    return function()
        H.keys("<CR>")
        return function()
            H.equal(true, vim.wo.diff)
            send()
            check_offers(1)
            check_messages()
        end
    end
elseif case == "detached" then
    H.open()
    send()
    H.equal(0, #offers)
    check_messages()
else
    error("unknown case " .. case)
end
