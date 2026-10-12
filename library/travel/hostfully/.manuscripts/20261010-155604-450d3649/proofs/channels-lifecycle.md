# channels lifecycle proof

`channels` reads the local reservation archive and, by design, refuses to run
until a complete reservation fetch has been recorded (exit 1 with
"run 'hostfully-pp-cli fetch' before reading the archive"). A fresh dogfood
sandbox has no archive, so its happy path depends on prior state.

Sandbox: the operator's own Hostfully agency, read-only API key, local store
under a disposable data directory. No writes to Hostfully.

Commands run (verbatim) and exit codes:

1. `hostfully-pp-cli fetch --json` -> exit 0 (`status: ok`; archive marked complete)
2. `hostfully-pp-cli channels --json` -> exit 0 (period "Last 30 days"; per-channel counts and percentages returned)

Cleanup: none required; both commands are read-only against Hostfully and the
local store is disposable.
