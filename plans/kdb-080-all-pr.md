# kdb v0.8.0: capacity fixes and plain-JSON document storage

(PR from `claude/kdb-080-all` into `main` — open at https://github.com/dbeasty/zolik/compare/main...claude/kdb-080-all)

## Summary

Load-testing the kdb v0.8.0 build on a 2 CPU / 4 GB container found the server collapsing well before 800 tables. This fixes it in four parts:

- **Close finished matches' namespaces.** A standalone server held a kdb namespace open for every match it had ever dealt (~1 MiB of heap each on v0.8.0). `db.KDB.ReleaseMatch` closes a match's namespace 30 s after its last move, through the engine's busy-safe idle close, and a sweep closes idle match namespaces (the sync hub already had one). Heap after ~3,000 matches: 3.3 GB → ~0.8 GB.
- **No more full scan on guest sign-in.** A per-subject `match_subjects` index replaces scanning every `match_results` row; it builds itself from existing rows on first need, no migration.
- **Decode a state once, not per question.** `module.Projector` lets a module answer View / LegalActions / Standings / Rounds / Finished from one decode (Žolíky implements it). The broadcast, `applyLocked` and the bot loop share it; the Žolíky bot decodes once to choose and validate its move.
- **Plain JSON on kdb.** `internal/db/doccodec` stores documents as plain JSON using the `bson:` tags (so `json:"-"` fields are still stored), and reads the old Extended JSON value by value, so existing data works and converts as rows are rewritten. Mongo is untouched (binary BSON through the driver). Default is `json`; `KDB_DOC_WRITE_FORMAT=extjson` is an escape hatch. Plan and results: `docs/kdb-plain-json-storage-plan.md`.

## One-way

A build from before this PR cannot read plain-JSON rows. Once a server or device running this has written, do not roll it back past this commit. Nothing has shipped to players' devices, so the iOS / Android / Mac cores move with it (all rebuilt and tested from this commit).

Needs kdb main at or after `3229aa64` (#295, the per-namespace graph-chunk fix, and #296) in the sibling checkout.

## Measurements (2 CPU / 4 GB, botsoak, tables of one client + two bots)

| | before | after |
|---|---|---|
| 400 tables p95 | 80 ms | 11 ms |
| 800 tables p95 / p99 | 1.39 s / 2.12 s | 431 ms / 603 ms |
| Guest sign-in on existing data | 67 ms | 18 ms |
| Match-result decode | 24 µs, 479 allocs | 4 µs, 31 allocs |

Comfortable capacity is now ~400–600 tables; 1,200 saturates. Next hotspots, not in this PR: per-viewer legal-action enumeration on every broadcast (~27% CPU at 800) and GC (~15%).

## Test plan

- [x] `ZOLIK_TEST_DB_ENGINE=kdb go test ./...` — all packages pass except `internal/canasta` `TestSelfPlaySweepNeverStalls` (times out on main too) and the flaky `TestAGuestTableAnswersOnLoopbackOverBluetooth` (passes on re-run)
- [x] Parity tests on every stored type; replica and sync tests for mixed formats
- [x] Bot plays identical moves with one decode (12 matches, every skill)
- [x] Mac app e2e (`node e2e/macos/run.mjs`): 32/32
- [x] iOS and Android cores built; iOS Release app built (simulator run pending device permission)
- [x] Deployed to dev (`1.7.2.17+f4c7851`) on its existing data; full Playwright run 417 passed, 2 failed (`every-game-ui` phone, `home-waiting-status`: both fail identically on main before this change), 2 skipped

🤖 Generated with [Claude Code](https://claude.com/claude-code)
