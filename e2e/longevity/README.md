# Longevity

A fleet of real browsers, signing in as real guests, playing this app through
its own screens for as long as you leave them running.

```sh
ZOLIK_SOAK_FOR=4h ./scripts/dev-stack.sh soak
```

## Why this is not another e2e spec

The suite next door answers *does it work*. It opens a page, plays a hand,
asserts something and throws the page away — which is the right shape for a
question about correctness and the wrong shape for every question that needs
time to answer. A listener added on every match and removed on none costs a
thirty-second spec nothing. A WebSocket room that is never emptied costs it
nothing. Memory that only goes up costs it nothing, because the server it ran
against was started ninety seconds ago.

This asks the other question: *does it still work after four hours of it*, with
several people playing at once, tables opening and closing, pages reloaded
mid-hand, and a server nobody has restarted in between.

So it fails on very little. An uncaught exception in somebody's browser, a
server error the server did not choose to send, or a fleet that never managed to
press a single control. Everything else — stalls, refusals, memory, console
noise — goes in the report to be read, because a soak run's output is a shape
rather than a verdict, and a run that aborted on the first hiccup would have
thrown away the four hours that were the point of starting it.

## What the clients do

Each client is its own browser context: its own storage, its own guest account,
its own sockets. They come in two shapes.

**Solo** — signs in through `/auth/guest` like anyone else, picks a game off the
lobby, seats bots, and plays. When the match ends it presses **Play again** a
few times, which is the path that keeps one page alive across dozens of matches
and is where a client that leaks per match shows it. Occasionally it reloads
mid-hand instead, so the board has to come back from the server and the socket
has to be re-established around a position that already exists. About one visit
in ten it goes and stands in the waiting room instead of playing, holding a
lobby socket open and pressing nothing — the only scenario with no match in it,
and the only one that would notice a waiting room that leaks an entry per visit.

**Duo** — two clients at one table, which is the shape no bot table reaches. One
opens a table and reads the join code off its own screen; the other types that
code into `/lobby/join`; the host seats bots until the server stops refusing to
deal, and both then play the same match at once. Two sockets in one room, every
action fanned out to the other.

Nothing here names a game. The list comes from the lobby, which renders itself
from `/modules`, so a module registered tomorrow is soaked tomorrow.

## How a client decides what to press

Every press lands on a real control in a real browser. There is no back channel
that submits actions and no seeded state.

It does read the match document over HTTP — the same one the screen is already
showing it — to answer one question the DOM cannot: *what would this control
take?* That is a deliberate departure from `generic-shell.spec.ts`, which
presses whatever is live and nothing else, and is what proves the shell needs no
knowledge of any game. A soak client that did only that would spend eight hours
stuck on the first hand of Žolíky, because a discard is not live until a card is
picked and nothing on screen says which control wanted a card. So this asks,
picks that many cards in the hand, and presses. It is not re-proving the shell is
game-agnostic; that is proven next door. It is keeping a table busy overnight.

Which control, among those that are live, is random rather than first. "First"
means one control per game is pressed ten thousand times and the rest never,
and reaching states a scripted order never reaches is the whole value of running
for hours. The one bias is against the controls that put the board back — undo,
reset turn — which pressed as often as anything else turn a match into an undo
loop that generates traffic and no progress.

**Every match has a budget.** When it runs out, or when the board has not moved
for a while, the client writes down what happened, leaves, and starts another.
This is why the run can be left alone: every game here can reach a position the
naive presser cannot get out of, and rather than trying to be good enough at six
games never to be wedged, a wedged client notices and goes elsewhere.

## What is measured

Three things, and they are what make this longevity rather than merely busy.

**The server's own opinion of itself** — `/healthz/capacity`, sampled
throughout. That endpoint is not a scrape: it is the admission controller's live
snapshot, the very numbers it uses to decide whether to let the next player in.
Its refusal counters are the difference between "the fleet went quiet" and "the
fleet was turned away". Its memory fraction is *not* a leak detector, which the
next paragraph exists because of.

**The server's live heap**, every few samples, from `/debug/memory?gc=1` — the
debug endpoint collects and hands pages back before answering, so what it
reports is reachable data rather than data the collector has not got to. This is
the only reading here a leak cannot hide in. A rising memory fraction is the
expected shape of a healthy server running under GOMEMLIMIT, which is told it
may grow toward its ceiling before working hard; a rising *live* heap at steady
offered load is not. The endpoint is shut on any public host, and a run against
one simply goes without this section.

**Each browser's heap**, sampled on the same tick. The client leaks too, and a
screen that has been open for six hours through two hundred matches is a thing
no spec here has ever looked at. The reading comes from devtools rather than
from `performance.memory`, which the browser quantises and caches — four clients
over seven minutes reported the same three figures they started with, which is
indistinguishable from a heap that never moved.

### A 503 is not a failure

The first run of this harness drove a 1 GiB dev container to its memory
high-watermark in three minutes, at which point the admission controller did
exactly what it is for: closed the waiting room, then match starts, then
gameplay sockets, and answered 503. Counting that as a server error would fail
every soak run that succeeded in applying enough load, which is backwards.

So 503 is counted separately, as back-pressure, and always reported rather than
failed on — along with how many samples the server spent closed to new
gameplay, because a fleet that was being turned away was not playing and its
match counts measure the refusal rather than the app. A 500, a 502 or a 504 is
still a failure: none of those is anything the server chose to say.

## Reading the report

Each run writes `reports/<timestamp>/report.md` and `run.json` (every sample and
every error, for when the shape of a curve matters). The report opens with a
verdict, then what the fleet got through, then per-client rows, then the server,
then a table of everything that went wrong — grouped, so that eight hours of one
recurring message is one row with a count instead of nine thousand lines.

The number to look at first is the memory line. A server that has just started
rises too, filling its caches, so the reading that means something is a run long
enough for that rise to have flattened out — and this one still climbing at the
end.

## Settings

All environment variables; the run is meant to be reproducible from one line of
shell history.

| Variable | Default | What it does |
|---|---|---|
| `ZOLIK_SOAK_FOR` | `10m` | How long the fleet plays. `90s`, `45m`, `2h`, `1h30m`; a bare number means minutes. |
| `ZOLIK_SOAK_SOLO` | `3` | One-person clients. |
| `ZOLIK_SOAK_DUOS` | `1` | Two-person tables. Each costs two browser contexts. |
| `ZOLIK_SOAK_GAMES` | *(all)* | Comma-separated module ids. Empty means whatever the lobby offers. |
| `ZOLIK_SOAK_MATCH_BUDGET` | `4m` | How long one match may run before the client abandons it. |
| `ZOLIK_SOAK_MATCH_MOVES` | `400` | And how many presses. |
| `ZOLIK_SOAK_STALL` | `90s` | How long a client may sit with nothing to press before that is a stall. |
| `ZOLIK_SOAK_SAMPLE` | `15s` | How often the server and the heaps are sampled, and how often a progress line is printed. |
| `ZOLIK_SOAK_LIVE_HEAP_EVERY` | `5` | Take a live-heap reading every this many samples. It stops the world, so it is deliberately sparse; `0` turns it off. |
| `ZOLIK_SOAK_HEADED` | `0` | Show the browsers. |
| `ZOLIK_SOAK_SLOWMO` | `0` | Milliseconds between interactions, for watching a headed run. |
| `ZOLIK_SOAK_REPORT_DIR` | *(per run)* | Write the report somewhere specific. |
| `ZOLIK_SOAK_FAIL_ON_PAGE_ERROR` | `1` | Fail on an uncaught exception in a client's page. |
| `ZOLIK_SOAK_FAIL_ON_SERVER_ERROR` | `1` | Fail on a 5xx that is not a 503. |
| `ZOLIK_SOAK_FAIL_ON_MEMORY_GROWTH` | `0` | Fail if the server's memory fraction rose by more than this (`0.15` = fifteen points of its limit). Off by default — see the note above about caches. |
| `ZOLIK_E2E_API_BASE` | `http://127.0.0.1:8090` | Shared with the e2e suite. |
| `ZOLIK_E2E_WEB_BASE` | `http://127.0.0.1:8114` | Shared with the e2e suite. |

## Running it

The stack has to be up already — the same one `dev-stack.sh up` starts:

```sh
./scripts/dev-stack.sh up
ZOLIK_SOAK_FOR=4h ZOLIK_SOAK_SOLO=6 ZOLIK_SOAK_DUOS=2 ./scripts/dev-stack.sh soak
```

Watch it happen, slowly, with one client:

```sh
ZOLIK_SOAK_HEADED=1 ZOLIK_SOAK_SLOWMO=250 ZOLIK_SOAK_SOLO=1 ZOLIK_SOAK_DUOS=0 \
  ZOLIK_SOAK_FOR=5m ./scripts/dev-stack.sh soak
```

Overnight, detached, on one game:

```sh
ZOLIK_SOAK_FOR=8h ZOLIK_SOAK_GAMES=canasta nohup ./scripts/dev-stack.sh soak > soak.log 2>&1 &
```

Or from here, without the wrapper:

```sh
cd e2e && ZOLIK_SOAK_FOR=30m npx playwright test --config longevity/longevity.config.ts
```

> **Bots answer at the pace the container is running at.** Unlike
> `dev-stack.sh test`, `soak` does not hurry them and does not recreate the
> container to do it: a run is meant to look like use, and a container restarted
> at the start of one would reset the very memory curve the run exists to
> measure. To pack more matches into an hour, start the stack with
> `ZOLIK_BOT_THINK_MIN_MS=400 ZOLIK_BOT_THINK_MAX_MS=1300 ./scripts/dev-stack.sh up`
> first.

## The first thing it found

Worth reading before trusting any memory number, because it is the shape this
mistake takes.

The first runs said the server could hold about four clients: the container went
from 48% of its 1 GiB to the 90% admission watermark in ninety seconds and
started refusing. That was true, and the reason was not load. A heap profile
(`/debug/pprof/heap?gc=1`) put 98% of the live heap in one call:
KDB's rescue reserve, a real page-touched allocation the engine holds back so it
can abort cleanly — taken once per storage namespace, and this server opens
nine. 432 MiB, live and uncollectable, before anybody connected. With the
collector then aiming at twice the live heap, the process idled just under the
watermark and the first arrivals pushed it over.

Sliced across the namespaces (`server/internal/db/kdb.go`), idle went from
831 MiB to 104 MiB and ten clients now run at a third of a 2 GiB container.

Two lessons are built into the harness because of it. A 503 is back-pressure
rather than a fault, and the memory fraction alone is not evidence of anything —
which is what the live-heap reading in *What is measured* is for.


## How big a fleet

One browser process, one context per client. On a laptop, ten to twelve contexts
is comfortable as far as *this* machine is concerned; past that the machine
running the test becomes the bottleneck and the run measures Chromium rather
than the server.

If the goal is thousands of connections rather than a long, honest UI session,
this is the wrong instrument: that is a protocol-level load generator, and it
would not be driving the screens. What this is for is the other half of the
question — whether an hour of real use leaves the server where it started.

## The files

| | |
|---|---|
| `soak.spec.ts` | The entry point: builds the fleet, runs it to the deadline, writes the report. |
| `lib/crew.ts` | What a solo client and a duo table each do, over and over. |
| `lib/player.ts` | One virtual person: signing in, and playing whatever match their page is on. |
| `lib/ui.ts` | The only file that knows what anything on screen is called. |
| `lib/watch.ts` | The server's capacity snapshot, and the browsers' heaps. |
| `lib/report.ts` | Collecting it all, judging it, and writing it down. |
| `lib/settings.ts` | Every environment variable, in one place. |
