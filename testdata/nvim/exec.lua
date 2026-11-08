-- Every process the plugin runs fails the same way: its stderr, else
-- "<what> failed (exit N)". The stub executables on PATH exit with the
-- code their name's caller expects, silently or with stderr.
local cli = require("revisa.cli")

local out, why = cli.exec({ "sh", "-c", "printf hi" }, {}, "probe")
H.equal("hi", out)
H.equal(nil, why)
out, why = cli.exec({ "sh", "-c", "exit 3" }, {}, "probe")
H.equal(nil, out)
H.equal("probe failed (exit 3)", why)
out, why = cli.exec({ "sh", "-c", "echo ' oops ' >&2; exit 1" }, {}, "probe")
H.equal(nil, out)
H.equal("oops", why)
out, why = cli.exec({ "/nonexistent/revisa-probe" }, {}, "probe")
H.equal(nil, out)
assert(why and why ~= "", "a missing executable gives a reason")
local dir = vim.uv.fs_realpath(vim.env.REVISA_STUBS)
out = cli.exec({ "pwd" }, { cwd = dir }, "pwd")
H.equal(dir .. "\n", out)

local messages = H.notifications()
H.equal(nil, cli.run({ cwd = dir }, { "list" }))
H.error(messages, "revisa failed (exit 6)")

out, why = require("revisa.notify.rex").run({ "list" })
H.equal(nil, out)
H.equal("rex failed (exit 4)", why)

H.equal(nil, require("revisa.notify.recado").targets(dir))
H.error(messages, "recado agents failed (exit 5)")

-- Every JSON payload is decoded the same way, naming where it came from.
local data, bad = cli.decode('{"a":1}', "probe")
H.equal({ a = 1 }, data)
H.equal(nil, bad)
data, bad = cli.decode("{", "probe")
H.equal(nil, data)
assert(bad:find("revisa: invalid JSON from probe: ", 1, true), bad)
