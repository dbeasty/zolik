# kdb v0.8.0: findings from running zolik on it

Found 2026-10-14/15 while moving zolik from kdb v0.7.1 to v0.8.0 (commit
`b85e4cc8`; the v0.8.0 tag has since moved to `b3b47d0e`, which differs only in
`release.yml` and `stream_resume_test.go`). zolik embeds kdb through
`embed.OpenFileHost` and `FileRuntimeOptionsFromEnv`, and opens **one namespace
per match** (thousands of small namespaces, a few dozen commits each).

## 1. Fixed 1 MiB commit-graph chunk per namespace (regression, high)

**Symptom.** Heap grows about 1.5 MB per namespace opened, against about 0.5 MB
on v0.7.1. At 2,200 opened match namespaces the process held 3.3 GB live heap
(after GC, idle, flat) and was heading into a 4 GB container limit.

**Evidence.** `pprof -sample_index=inuse_space`: 2,290 MB of 3,196 MB
(72%) in `dag.(*nodeStore).put`, cum 2,266 MB under `dag.NewInMemoryCommitDag`
via `embed.(*Host).openNamespace`. `put` appends a chunk of
`slotsPerChunk * graphRecordSize` = 4096 x 256 B = **1 MiB**
(`go/kdb/dag/node_store.go:136`, `graph_record.go:40`) the first time a
namespace commits, whatever its size. 2,290 MB / ~2,200 namespaces is 1.04 MiB
each.

**Same load, same machine, same commit of zolik** (`botsoak`, 300 tables twice):

| kdb | live heap after ~600 matches |
|---|---|
| v0.7.1 | 299 MB |
| v0.8.0 defaults | ~740 MB at ~400 matches; 3.3 GB at ~2,200 |
| v0.8.0, `KDB_GRAPH_FILE=off KDB_GRAPH_MEMORY=go` | 389 MB |

So the opt-out restores v0.7.1's footprint, which isolates the cost to the new
chunked node store, not to anything in zolik.

**Ask.** Do not allocate a full chunk for a namespace that holds a handful of
commits: start small (first chunk sized for a few dozen slots, doubling to
`slotsPerChunk`), or keep tiny graphs in the Go map until they cross a
threshold. The release notes' "memory stays flat as data grows" holds for one
big namespace; the many-small-namespaces case is the opposite and is what
`docs`/benchmarks do not cover. Please add a benchmark: open N=5,000 namespaces,
write 20 commits to each, report heap per namespace, for both
`GraphMemoryKDB` and `GraphMemoryGo`.

**Related, probably also worth a look.** `docHashShard.newChunk` is the next
largest at 350 MB (11%), plausibly the same shape (per-namespace fixed
allocation). `rescueReserve.allocate` is 48 MB and was already there.

## 2. An sstable that neither v0.7.1 nor v0.8.0 can compact (data, medium)

`ns/zolik/sessions/sstable/L4/60d3f762-1112-4271-8982-756435ac63d9`, 47,983
bytes, written 2026-10-07 by a v0.7.x build, in a 393 MB dev volume.

* v0.8.0 logs `background sstable merge failed (... footer overruns the file)`
  (`sidecar.go:513`, `appendIndexEntries`).
* v0.7.1 logs `could not compact the blob store (... negative offset or length)`.
* Reads and writes work; only compaction of that namespace is stuck.
* The file's last 4 bytes (the footer trailer, read as `indexLen`) are
  `1b 36 43 58`, i.e. about 456 million, which cannot be a footer length. Either
  the trailer was written wrong or the file was damaged afterwards.

**Ask.** Decide whether v0.7.x could write this trailer (a writer bug that is
now in data in the wild) or whether it is damage. If it is a writer bug, the
reader should either handle it or `kdb-inspect` should be able to rewrite it;
`verify` today only walks the delta log. I can hand over the file and the
volume copy (`zolik-dev_kdb_data_pre080`).

## 3. Release gate flake (low, already known to your release owner)

`TestStreamFarBehindOverTheFrameLimitCatchesUp`: `recordingConn.waitAt` has a
fixed 5 s deadline (`stream_resume_test.go:72`) that is too tight on the Linux
runner under `-race` (test took 12 s there; 3.3 s locally). Not a catch-up bug.
The tag has already been re-cut for this.

## Repro for 1

```bash
# zolik server image built from zolik main + kdb v0.8.0, 2 CPU / 4 GB
docker run -d --cpus 2 --memory 4g -e ENABLE_DEBUG_ENDPOINTS=true ... zolik-dev-app
go run ./cmd/botsoak -base http://127.0.0.1:PORT -tables 300 -game zolik -seats 3 -for 90s
curl -s 'http://127.0.0.1:PORT/debug/memory?gc=1' | grep heapAllocBytes
go tool pprof -sample_index=inuse_space -top <(curl -s PORT/debug/pprof/heap)
# repeat with  -e KDB_GRAPH_FILE=off -e KDB_GRAPH_MEMORY=go
```

## What zolik is doing on its side

zolik never closes a finished match's namespace in standalone mode (idle close
is wired only for the sync hub role). That is zolik's to fix and makes #1
worse, but #1 is a regression either way: v0.7.1 under the same behaviour used
a third of the memory. Until kdb changes, zolik will ship with
`KDB_GRAPH_FILE=off KDB_GRAPH_MEMORY=go`.
