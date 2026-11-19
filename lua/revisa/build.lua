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
-- false, or when the plugin is not its own git checkout. It is building
-- from the moment it starts, so a command run while the check is still
-- looking reports that instead of a missing CLI.
function M.check()
    if vim.g.revisa_auto_build == false then
        return
    end
    building = true
    local ok = pcall(vim.system,
        { "git", "rev-parse", "--show-toplevel", "HEAD" },
        { cwd = root, text = true }, function(result)
            if result.code ~= 0 then
                building = false
                return
            end
            local top, rev = unpack(vim.split(vim.trim(result.stdout), "\n"))
            vim.schedule(function()
                local real = vim.uv.fs_realpath(root)
                if not rev or vim.uv.fs_realpath(top) ~= real
                    or built(rev) then
                    building = false
                    return
                end
                build(rev)
            end)
        end)
    if not ok then
        building = false
    end
end

function M.building()
    return building
end

return M
