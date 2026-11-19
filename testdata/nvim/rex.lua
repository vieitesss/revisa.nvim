local case = vim.env.REVISA_CASE

local messages = H.notifications()
local offers, picks = {}, {}
for pick in (vim.env.REVISA_PICK or "1"):gmatch("[^,]+") do
    picks[#picks + 1] = pick
end

local setting = vim.env.REVISA_SETTING or ""
if setting ~= "" then
    require("revisa").setup({ notify = setting })
end

local function check_recipients(offer)
    assert(offer, "picker did not open")
    H.equal(vim.split(vim.env.REVISA_PANES, "\n", { plain = true }),
        offer.items)
end

local function check_confirm(offer)
    assert(offer, "confirm did not open")
    H.equal("revisa: send to " .. vim.env.REVISA_SENT .. "?", offer.prompt)
    H.equal({ "send", "cancel" }, offer.items)
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

local function check_sent()
    local last = messages[#messages]
    assert(last, "no notification after send")
    H.equal("revisa: sent to " .. vim.env.REVISA_SENT, last.message)
    H.equal(vim.log.levels.INFO, last.level)
end

if case == "unavailable" then
    vim.cmd("Revisa send")
    H.error(messages, vim.env.REVISA_ERROR)
    H.equal(0, #offers)
elseif case == "send" then
    H.open()
    vim.cmd("Revisa send")
    H.equal(2, #offers)
    check_recipients(offers[1])
    check_confirm(offers[2])
    check_sent()
elseif case == "cancel" then
    H.open()
    vim.cmd("Revisa send")
    H.equal(2, #offers)
    check_recipients(offers[1])
    check_confirm(offers[2])
    for _, message in ipairs(messages) do
        assert(not message.message:find("sent to", 1, true),
            "cancel sent: " .. message.message)
    end
else
    error("unknown case " .. case)
end
