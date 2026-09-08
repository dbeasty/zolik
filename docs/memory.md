# Memory

Where this server's memory goes, what decides when it refuses players, and the
two things that were wrong with that when it was first measured.

Written because the question "how many people can this hold" turned out to be
unanswerable from anything the server said about itself, and because the first
answer it appeared to give — about four — was wrong for a reason worth writing
down.

## Four thresholds, and their order

Every one of these is a fraction of the container's memory limit, so all four
move together when `mem_limit` changes. They live in four different files
because each belongs to a different mechanism, and no file shows them in a row —
which is why `/debug/memory` reports them together.

| At | What happens | Set in |
|---|---|---|
| 0.50 | KDB's hot-tier cache pool: the ceiling every namespace draws its cache from | `kdbHotTierFraction`, `server/internal/db/kdb.go` |
| 0.80 | GOMEMLIMIT: the collector starts working harder | `goMemLimitFor`, `server/internal/app/app.go` |
| 0.85 | The admission gate refuses new gameplay | `ADMISSION_MEMORY_WATERMARK` |
| 0.85 | KDB sheds writes with `SERVER_BUSY` | `kdbMemoryRejectFraction`, `server/internal/db/kdb.go` |

The order of the middle two is the part that matters, and it used to be the
other way round. GOMEMLIMIT was a flat 0.9 against a 0.85 gate, so the collector
was told it could grow *past* the point where the gate starts turning players
away — and under sustained allocation it did. Soak runs show the process
climbing to 91% while refusing arrivals, and a single collection then giving
280 MiB back. People were refused to protect memory that was garbage.

Collector first, gate second: the runtime is pushed to work at a line below the
one the gate defends, so a player is refused only when collecting could not get
under it. That is the only circumstance in which refusing somebody is the right
answer.

It is a margin, not a guarantee. GOMEMLIMIT governs the Go runtime's own
accounting; the gate reads the cgroup, which also counts file pages the runtime
knows nothing about. Ordering them correctly is not the same as making them
agree.

## What was actually using the memory

### The rescue reserve, nine times over

`KdbServerRuntime.SetMemoryLimit` takes the engine's default rescue reserve —
48 MiB, and a *real* allocation with every page touched, because a reservation
the allocator has not honoured yet is an intention rather than headroom. This
server opens one storage runtime per namespace, and it has nine.

**432 MiB, live and uncollectable, before a single player connected.** 42% of a
1 GiB container. With the collector then aiming at twice the live heap, the
process idled at roughly 84% of that container, a hair under the gate, and the
first few arrivals pushed it over. That is the whole of the "it can only hold
four people" result.

Fixed by `kdbRescueReserveBytes` — one reserve's worth divided across the
namespaces, so each still has a share to drop when its guard goes critical, and
the total is what a single-runtime deployment holds. One process has one memory
ceiling and needs one reserve's worth of room to abort within.

Idle went from 831 MiB to 104 MiB. `TestRescueReserveIsSharedAcrossNamespaces`
is what stops a tenth namespace quietly adding another 48 MiB.

This is the same shape as the earlier per-namespace memory budget fix (d0ee7ef).
Consolidating onto one `embed.Host` (ad5195c) shared the data root, the lock and
the hot-tier pool, but not the per-runtime `Admission` — which is where the
reserve lives. **Anything constructed inside that namespace loop costs nine
times what it looks like.**

### After that, it is cache

With the reserve fixed, a 22-minute soak of ten browser clients took the live
heap from 60 MiB to 623 MiB — measured after a forced collection, so this is
reachable data rather than uncollected garbage. A heap profile puts 89% of it in
`kdbNamespace.put`: the document copy handed to `embed.PutJSONDocument` on every
write.

Three things say what it is, and what it is not:

- **It is not per-connection.** Ten minutes after every client disconnected it
  was still 623 MiB, with goroutines back to their idle 27. Nothing was leaked
  along with the sockets.
- **It is not resident data.** Restarting the process against the same volume
  came back to 59 MiB. Nothing in that 623 MiB had to be in memory.
- **It is bounded, in principle, by a number we chose.** The hot-tier pool is
  half the container — 1 GiB of the 2 GiB stack — and the growth is the shape of
  a cache filling toward its budget.

What has *not* been shown is the plateau. The run ended at 623 MiB and rising,
short of the 1 GiB the pool is allowed. A longer soak would settle it, and the
longevity harness now samples live heap on purpose so that it can.

## How to measure it yourself

`/debug/memory` and `/debug/pprof/*`, behind `ENABLE_DEBUG_ENDPOINTS` — on under
a local `APP_ENV`, a literal `false` in `deploy/compose/zolik.yml`, and opened
for dev by `scripts/dev-stack.sh`. It exposes the shape of the process and can be
asked to stop the world, so it is not something a public listener carries.

```sh
# Where the memory is, split into live objects, garbage, and page cache.
curl -s localhost:8090/debug/memory | jq

# The same, after collecting first: what is left is live by definition.
curl -s 'localhost:8090/debug/memory?gc=1' | jq '.go.heapAllocBytes'

# And which call sites are holding it.
curl -s 'localhost:8090/debug/pprof/heap?gc=1' -o /tmp/heap.pb.gz
cd server && go tool pprof -top -sample_index=inuse_space /tmp/heap.pb.gz
```

**Always take the `?gc=1` reading before concluding anything.** `/healthz/capacity`
reports one number, and one number cannot separate live objects from garbage the
collector has not reached from file pages the kernel is holding on our behalf.
Those have completely different answers: garbage means the collector is doing
what it was told, file pages mean nothing is wrong at all, and only live objects
mean somebody has to go and look. A heap sitting under GOMEMLIMIT looks exactly
like a leak until you collect.

For the shape over hours rather than a moment, `scripts/dev-stack.sh soak` —
see [`e2e/longevity/README.md`](../e2e/longevity/README.md).

## What this means for scale

Connections are cheap. The project's own measured figure is ~65 KiB per
connected player (`admissionPerConnBytes`), and the soak agrees with it:
goroutines stayed flat at ~65 across ten clients and returned to 27 when they
left. A thousand idle sockets is tens of megabytes.

What costs memory is write volume, held in a cache sized as a fraction of the
container. That is a very different scaling question from "how many users", and
it is the one to answer before sizing anything for thousands: the cache will
expand to its budget regardless of how many people are online, so total memory
is not a measure of how many users fit.
