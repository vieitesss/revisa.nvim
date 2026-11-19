local api = vim.api
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

-- marked counts the entries that say they match the worktree, and
-- asserts each one came first.
local function marked(offer)
    local seen = 0
    for position, item in ipairs(offer.items) do
        if item:find("(matches this worktree)", 1, true) then
            seen = seen + 1
            H.equal(1, position)
        end
    end
    return seen
end

-- check_panes asserts one picker offered the expected panes, in order,
-- with the expected number of them marked.
local function check_panes(number, expected, matched)
    local offer = offers[number]
    assert(offer, "picker " .. number .. " did not open")
    H.equal(vim.split(expected, "\n", { plain = true }), offer.items)
    H.equal(matched, marked(offer))
end

-- check_preview asserts the INFO notification that shows a pane.
local function check_preview(text)
    for _, message in ipairs(messages) do
        if message.message == text
            and message.level == vim.log.levels.INFO then
            return
        end
    end
    error("no preview of the pane: " .. vim.inspect(messages))
end

-- check_confirm asserts the confirm that follows a preview.
local function check_confirm()
    local offer = offers[#offers]
    assert(offer.prompt:find("send to", 1, true), offer.prompt)
    H.equal({ "send", "cancel" }, offer.items)
end

-- vim.ui.select answers with the next REVISA_PICK entry, recording what
-- the picker offered.
vim.ui.select = function(items, options, on_choice)
    offers[#offers + 1] = { items = items, prompt = options.prompt }
    if #offers == 2 then
        check_preview(vim.env.REVISA_PREVIEW)
    end
    local pick = picks[#offers]
    if pick == nil or pick == "cancel" then
        on_choice(nil, nil)
        return
    end
    local index = tonumber(pick)
    on_choice(items[index], index)
end

if case == "unavailable" then
    vim.cmd("Revisa send")
    H.error(messages, vim.env.REVISA_ERROR)
    H.equal(0, #offers)
elseif case == "send" then
    H.open()
    vim.cmd("Revisa send")
    H.equal(2, #offers)
    check_panes(1, vim.env.REVISA_PANES, tonumber(vim.env.REVISA_MARKED))
    check_confirm()
    local last = messages[#messages]
    H.equal("revisa: sent to block:01AGENT", last.message)
    H.equal(vim.log.levels.INFO, last.level)
elseif case == "cancel" then
    H.open()
    vim.cmd("Revisa send")
    H.equal(2, #offers)
    check_confirm()
    for _, message in ipairs(messages) do
        assert(not message.message:find("sent to", 1, true),
            "cancel sent: " .. message.message)
    end
else
    error("unknown case " .. case)
end
