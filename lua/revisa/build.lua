-- The plugin builds its own CLI: on load it compares the commit its
-- checkout is at with the commit of the last build, and rebuilds and
-- reinstalls when they differ. This works the same under any plugin
-- manager, and after a plain git pull.
local M = {}

local source = debug.getinfo(1, "S").source:sub(2)
local root = vim.fn.fnamemodify(source, ":h:h:h")
local stamp = root .. "/bin/.revisa-rev"

local building = false

local function notify(message, level)
    vim.schedule(function()
        vim.notify(message, level or vim.log.levels.INFO)
    end)
end

local function failed(message)
    building = false
    notify("revisa: building the CLI failed: " .. message
        .. "\nSet vim.g.revisa_auto_build = false to build it yourself.",
        vim.log.levels.ERROR)
end

-- spawn runs command in the checkout and calls done with its result, or
-- reports why it could not start.
local function spawn(command, done)
    local ok, err = pcall(vim.system, command,
        { cwd = root, text = true }, done)
    if not ok then
        failed(tostring(err))
    end
end

local function why(result, what)
    local message = vim.trim(result.stderr or "")
    if message == "" then
        message = what .. " exited with code " .. result.code
    end
    return message
end

local function build(rev)
    building = true
    notify("revisa: building the CLI…")
    spawn({ "go", "build", "-o", "bin/revisa", "./cmd/revisa" },
        function(result)
            if result.code ~= 0 then
                failed(why(result, "go build"))
                return
            end
            spawn({ root .. "/bin/revisa", "install" }, function(installed)
                if installed.code ~= 0 then
                    failed(why(installed, "revisa install"))
                    return
                end
                local file = io.open(stamp, "w")
                if file then
                    file:write(rev, "\n")
                    file:close()
                end
                building = false
                notify("revisa: CLI built")
            end)
        end)
end

local function built(rev)
    local file = io.open(stamp, "r")
    if not file then
        return false
    end
    local last = vim.trim(file:read("*a") or "")
    file:close()
    return last == rev and vim.fn.executable(root .. "/bin/revisa") == 1
end

-- check builds the CLI in the background when the checkout's commit is
-- not the one last built. It does nothing when vim.g.revisa_auto_build is
-- false, or when the plugin is not its own git checkout.
function M.check()
    if vim.g.revisa_auto_build == false then
        return
    end
    pcall(vim.system, { "git", "rev-parse", "--show-toplevel", "HEAD" },
        { cwd = root, text = true }, function(result)
            if result.code ~= 0 then
                return
            end
            local top, rev = unpack(vim.split(vim.trim(result.stdout), "\n"))
            vim.schedule(function()
                local real = vim.uv.fs_realpath(root)
                if not rev or vim.uv.fs_realpath(top) ~= real
                    or built(rev) then
                    return
                end
                build(rev)
            end)
        end)
end

-- building tells whether a build started by check is still running.
function M.building()
    return building
end

return M
