-- The review-beside feature: a second editor opens next to an agent pane,
-- in the worktree under review, so the reviewer reads the code where the
-- agent works. Block ids are discovered on every call and never stored:
-- they expire with the pane.

local M = {}
local cli = require("revisa.cli")
local notify = require("revisa.notify")
local review = require("revisa.review")
local rex = require("revisa.notify.rex")

-- prompt asks which pane the review opens beside.
local prompt = "revisa: open the review beside"

-- split opens a pane beside target that runs the review in worktree.
local function split(target, worktree)
    local _, why = rex.run({ "split", "-s", vim.env.REX_SESSION,
        "-b", target.id, "-c", worktree, "--",
        "nvim", "+Revisa review " .. worktree })
    if why then
        cli.error(why)
    end
end

-- M.open lists the panes of the session and opens the review beside the
-- one the reviewer picks.
function M.open(worktree)
    local targets = rex.targets(worktree)
    if not targets then
        return
    end
    if #targets == 0 then
        cli.error("revisa: no rex pane to open the review beside")
        return
    end
    notify.pick(targets, prompt, function(chosen)
        split(chosen, worktree)
    end)
end

-- M.beside opens the review beside an agent pane, in the chosen
-- worktree. Without one, the worktree picker runs first.
function M.beside(worktree)
    local ready, why = rex.available()
    if not ready then
        cli.error(why)
        return
    end
    if worktree == nil or worktree == "" then
        review.pick(function(path)
            M.open(path)
        end)
        return
    end
    M.open(worktree)
end

return M
