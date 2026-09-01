# AI skill levels — plan, and what the measurements did to it

> **Status: implemented.** The sections below are the plan as written; §10 at the
> end records what survived contact with a benchmark, which was less than half of
> it. Read §10 first if you want the outcome rather than the reasoning.


Make the Žolíky opponents genuinely stronger, genuinely weaker, and let the host pick which
when the table is created.

- **Baseline:** `main` @ `c09c37d`
- **Deliverable:** a `botSkill` option on the create-game screen; four levels that measurably
  differ; a benchmark command and a CI gate that make "smarter" a number rather than a claim.

---

## 1. What is actually there today

The difficulty ladder exists. Almost none of it is connected.

| Piece | State |
|---|---|
| `ai.HeuristicAgent` carries a `difficulty` string | ✅ exists |
| Difficulty changes behaviour | only two discard signals (`dangerous`, `seenBefore`) |
| `zolikmod.Bot()` picks the difficulty | ❌ hardcoded `"medium"` — easy and hard are **unreachable in the product** |
| `ai.AINames` — per-difficulty bot names | ❌ referenced by nothing; bots are named `Bot XY` |
| `models.Player.AIDifficulty` | ❌ never written; `addBot` leaves it empty |
| `stats` buckets records by difficulty | ✅ ready, and every bot currently lands in `"unspecified"` |
| `visibleFor` → `discardHistory()` in `zolikmod/bot.go` | ❌ returns a map of nils, so the *only* signal that separates "hard" from "medium" **cannot fire** |
| `VisibleState.TotalScores` | ❌ dropped by `zolikmod.visibleFor` |
| Opponent hand counts | ❌ absent from `VisibleState` — although they are already public in the view (`zone.opponentHand`, `seat.cards`) |

So the honest summary: there is one bot, it plays at "medium", and one of medium's two
distinguishing signals is dead on arrival. Phase 1 is mostly plumbing, and it is small.

Read alongside: `internal/ai/heuristic.go` (the agent), `internal/zolikmod/bot.go` (the
adapter that starves it), `internal/match/bots.go` (the runtime loop that drives it),
`internal/module/descriptor.go` (how a lobby option is declared and rendered).

## 2. Where the level is chosen, and how it reaches the seat

The create-game screen (`client-react-native/app/lobby/games.tsx`) renders every option in
the module descriptor generically — label, then a pill per choice. **A declared option needs
no client work to appear.** `module.PauseOption()` is the precedent: a setting that is about
how a match is *presented* rather than about the rules, declared in `module` and appended by
each module's `Descriptor()`.

`botSkill` is exactly that shape.

### 2.1 Declare it (`internal/module/descriptor.go`)

```go
const OptBotSkill = "botSkill"

type Skill int
const (
    SkillEasy Skill = iota
    SkillMedium
    SkillHard
    SkillExpert
)

func BotSkillOption() OptionSpec { /* enum_int, four choices */ }
func (c MatchConfig) BotSkill(dflt Skill) Skill { ... }
```

Ints on the wire (the options protocol is `map[string]int`), a named type in Go, and the
existing lower-case strings (`easy` / `medium` / `hard` / `expert`) where a human or a stats
bucket has to read it. One conversion, in one file.

Each module appends `module.BotSkillOption()` in `Descriptor()` and names its own default —
the same freedom `PauseOption()` gives, and for the same reason: a poker bot's "hard" is not
a rummy bot's.

### 2.2 Carry it to the bot (`internal/module/bot.go`)

`Bot.Act` currently takes a bare `playerID string`. Widen that one parameter:

```go
type BotSeat struct {
    PlayerID string
    Skill    Skill
    Seed     int64  // derived from match seed + player id; see §5.3
}

type Bot interface {
    Act(s State, seat BotSeat, offers []ActionOffer) (Action, bool)
}
```

Blast radius: four implementers (`zolikmod`, `holdem`, `prsi`, `canasta`), one production
call site (`match/bots.go:129`), eight test call sites. Mechanical.

The alternative — an optional `SkilledBot` interface, leaving `Bot` alone — is more in
keeping with `Botted` and `RulesProvider`, but it forks the runtime's dispatch into two paths
that must stay in agreement, and buys nothing: every bot wants to know which seat it is
playing anyway. One seam, widened, is the better trade here.

### 2.3 Seat it (`internal/match/handlers.go`, `bots.go`)

- `addBot` accepts an optional `{"skill": "hard"}`. Absent → read `match.Options["botSkill"]`
  → absent → the module's default. Write the result to `models.Player.AIDifficulty`, which
  has been waiting for it, and which `stats.SubjectForPlayer` already keys on.
- Name the bot from `ai.AINames[skill]`, deduped against seats already taken. "Master
  Miroslav" versus "Rookie Rita" tells the player what they are up against with no new UI.
- `botLoop` builds `BotSeat` from the seated player. Empty `AIDifficulty` → medium, so every
  match created before this ships plays exactly as it does now.

Because `addBot` takes a per-seat skill, **mixed tables come free** — a table of one Rookie
and one Master is a client change with no server work. The create-game picker sets the
table-wide default; the table screen (`app/lobby/table.tsx`) can override per seat later.

## 3. Fix the senses before tuning the brain

The agent cannot be clever about what it cannot see. Three of these are bugs, not features.

1. **Real discard history.** The module has no action log — the runtime keeps one, opaquely —
   so `discardHistory()` returns nils and `rankAlreadyDiscardedByOthers` is always false.
   Fix in the module's own state: a `Discards map[string][]string` on `matchState`, appended
   in `Apply` on a discard verb. Exact, survives reload, in the module's own vocabulary.
   This revives the signal that already defines "hard".
2. **Hand counts.** Add `HandCounts map[string]int` to `VisibleState`. No information leak —
   it is already rendered on every client. This is what unlocks endgame play (§4.4).
3. **Total scores.** `zolikmod.visibleFor` drops `TotalScores`; pass it. Matters on the last
   deal, where the leader plays for safety and the trailer plays for the go-out.
4. **A no-peek test.** `holdem` has `TestBotDoesNotPeek`. Mirror it: hand the Žolíky bot a
   state with opponents' hands populated and assert its choice is unchanged when those hands
   are permuted. Cheap, and it is the guarantee that makes "stronger" honest rather than
   informed.

## 4. What actually makes it smarter

Ordered by strength-per-line. Each is a separate PR with self-play numbers attached (§5).

### 4.1 Keep-value instead of a meld/not-meld bit

`meldMaterialPositions` marks only cards in a **complete** meld, deliberately, because
protecting partials would freeze the hand. The consequence is the agent's single dumbest
habit: holding `K♥ K♠` and throwing one of them, because kings are worth ten points and
neither is in a finished meld.

Replace the binary `ownMeld` in `discardCandidate` with a scalar:

| Material | Keep value |
|---|---|
| In a complete, layable meld | maximum |
| One card off a meld (`K♥K♠K♦` minus one; `8♠9♠10♠` in a min-4 run) | high |
| Pair, or two-card run fragment | medium, scaled by how many outs are still live |
| Isolated | low, then points decide as today |

Scaled by outs is what stops the freeze: a pair of kings with three kings already on the
table is not worth keeping, and the agent can see that.

### 4.2 Choose the lay-off

`findLayOff` returns the **first** card that fits, in sorted-owner order — chosen for
determinism, not for value. Rank the candidates instead: shed the highest-penalty card first
while down, and skip a lay-off that opens a chain for the next seat when the deal is close to
ending. The reclaim-trap guards stay exactly as they are.

### 4.3 Plan the go-out

Going out is currently emergent: lay off, lay off, lay off, and eventually the hand empties.
When the hand is at or under a small threshold, search for a *full sequence* — lay-offs plus
new melds plus the final discard — that empties it this turn, and take it. Bounded search, on
a hand of ≤6, so the cost is nothing.

### 4.4 Endgame dumping

With `HandCounts` in hand: when any opponent is one or two cards from out, stop protecting
partial material and shed the most expensive card that is still safe. A held pair of kings is
worth twenty penalty points the moment somebody else goes out.

### 4.5 Joker economy

A joker is the most expensive card in the deal and the most useful card in the hand. Do not
spend one on a meld a natural completes; do not hold one past the point where the deal is
likely to end without you. Interacts with `JokerReclaimMustPlay` — the existing guards in
`findLayOffAmong` already model that correctly and must not be bypassed.

### 4.6 Draw evaluation

`discardPickupUseful` takes the top discard only when it lands *immediately*. Widen it: take
it when it turns a pair into a triple or extends a two-card run, weighed against the fact that
everyone can see you take it.

## 5. What makes it dumber — without making it broken

"Less smart" is a separate axis from "less capable", and this is where a weak bot usually
goes wrong: it stops playing, or it strands its own turn.

### 5.1 One profile struct, one table

```go
// internal/ai/profile.go
type Profile struct {
    Name string

    // Perception
    ReadTableDanger   bool  // don't feed a live meld
    ReadDiscardMemory bool  // avoid a rank opponents passed on
    ReadHandCounts    bool  // notice somebody is about to go out

    // Planning
    PlanInitialMeld bool    // full combinatorial plan search, vs. lay the first legal meld
    KeepPartials    int     // 0 none · 1 pairs and run fragments · 2 outs-weighted
    LayOffPolicy    Policy  // firstFit · highestPoints · pointsThenSafety
    PlanGoOut       bool
    JokerEconomy    bool

    // Fallibility
    BlunderRate float64  // choose the second-best discard instead of the best
    MissRate    float64  // fail to notice an available lay-off this turn
}
```

Every knob named in one place, four values of it in one table below. No `difficulty ==
"hard"` string comparisons scattered through `heuristic.go` — the two that exist today move
into `Profile` on the way past.

| | Easy | Medium (today) | Hard | Expert |
|---|---|---|---|---|
| Table danger | – | ✅ | ✅ | ✅ |
| Discard memory | – | – | ✅ | ✅ |
| Hand counts | – | – | ✅ | ✅ |
| Initial-meld plan | first legal meld | full search | full search | full search, values what it spends |
| Keep partials | none | none | pairs + fragments | outs-weighted |
| Lay-off policy | first fit | first fit | highest points | points, then safety |
| Go-out planning | – | – | ✅ | ✅ |
| Joker economy | – | – | – | ✅ |
| Blunder rate | 12% | 3% | 0 | 0 |
| Miss rate | 25% | 5% | 0 | 0 |

Medium is deliberately today's agent, unchanged. That keeps the existing tests meaningful and
gives every other level a fixed reference point to be measured against.

### 5.2 A blunder must never be a dead end

The engine has no pass. `handCanStillDiscard` exists because an agent that melds away its last
natural card wedges the deal for the whole table. So: **blunders are sampled only from moves
that are legal and that leave the turn completable.** Concretely, the blunder path picks the
second-best candidate from the *same* filtered list the best move came from — it never
widens the candidate set, never discards a joker it may not discard, never strands a pending
joker. A weak bot plays badly; it does not play illegally, and it does not stop the table.

### 5.3 Randomness that is still reproducible

`HeuristicAgent` is deterministic today, and three tests pin that. Blunder and miss rates need
an RNG, so: a seeded one, derived in the runtime from `match.Seed` + player id + turn number
and handed over in `BotSeat.Seed`. Same match, same replay, same play. The determinism tests
change from "there is no RNG" to "the same seed produces the same move", which is the property
they were actually protecting.

## 6. Measuring it — the load-bearing part

Nothing here is "smarter" without a scoreboard, and the harness is already half written.

1. **Promote the simulator.** `playDeal` in `internal/ai/selfplay_test.go` drives whole deals
   through the real engine with no database. Move it to `internal/ai/sim` so a command and the
   tests share one copy.
2. **`server/cmd/aibench`.** Round-robin: every profile against every other, N deals per
   pairing over a fixed seed set, across every rules profile (Classic, quota, clean-run on and
   off — `RulesConfig` varies enough that a gain under one can be a loss under another).
   Reports per profile: win rate, mean deal score, mean turns to go out, turn of first meld,
   and the three numbers that must be zero — rejections, stalls, stranded lays.
3. **The CI gate.** A test over a fixed seed set asserting monotonicity —
   `expert > hard > medium > easy` by win rate, with a margin wide enough not to flake — and
   zero rejections, stalls and stranded lays at *every* level. That single test is what turns
   this plan's claim into a fact, and what stops a later "improvement" from quietly inverting
   the ladder.
4. **An e2e that asserts what the player sees.** Pick a level on the create screen, start the
   table, and assert the seated bot's name and level are rendered. The option renders itself
   from the descriptor; that is exactly why it needs a test that looks at the screen.

## 7. Clients and rollout

- **React Native lobby** — nothing to write for the picker; verify it renders and that the
  chosen value reaches `createMatch`. Add the skill to the table screen's `addBot` when
  per-seat mixing is wanted.
- **TUI** — `client-tui/api/client.go:143` gains the optional skill parameter.
- **Stats** — `difficultyOrder` learns `expert`. Existing `easy`/`medium`/`hard` buckets keep
  their keys.
- **Server message keys** — if any new user-visible server string appears, regenerate
  `serverKeys.json` or CI fails.

## 8. Risks, named

- **Retuning `easy` changes what its historical stats bucket means.** Records from before and
  after are not comparable. Either accept it with a dated note in `internal/stats`, or ship
  the new weak level under a fresh id and retire the old one. Recommend accepting it: the old
  bucket is empty anyway, because `addBot` never wrote a difficulty.
- **Search cost.** The initial-meld search is already exponential in hand size; go-out
  planning adds another bounded search. `botMaxStall` and the loop's step budget are the
  backstop, but keep every new search explicitly bounded rather than relying on them.
- **A weaker bot must stay a *playing* bot.** §5.2 is the constraint, and the zero-stall gate
  in §6.3 is how it is enforced rather than hoped for.
- **Four levels is a guess.** If playtesting says the new `easy` is still too strong, a fifth
  below it is one row in the profile table and one choice in the option spec.

## 9. Order of work

| PR | Contents | Gate |
|---|---|---|
| 1 | `botSkill` option, `Skill`, `BotSeat`, `addBot` writes `AIDifficulty`, names from `AINames` | picker renders; medium behaviour byte-identical |
| 2 | Sensors: real discard history, hand counts, total scores, no-peek test | `hard` differs from `medium` for the first time |
| 3 | `Profile` struct + table; today's two string comparisons move into it | no behaviour change at medium |
| 4 | `internal/ai/sim` + `cmd/aibench` + monotonicity gate | the ladder is a number |
| 5 | Easy and the fallibility knobs (§5.2, §5.3) | easy loses to medium; zero stalls |
| 6–9 | Keep-value, lay-off choice, go-out planning, endgame dumping, joker economy | each raises hard/expert win rate against the fixed seed set |


---

## 10. What actually shipped

The plan's load-bearing idea was §6: build the scoreboard before the strength
work, so "smarter" is a number. That turned out to matter far more than any of
the individual improvements, because **most of the improvements were not
improvements.** Every claim below is a sweep of 150–200 whole deals through the
real engine, across three rulesets, at two and three seats
(`go run ./cmd/aibench`, and the gate in `internal/ai/sim`).

### 10.1 The ladder that shipped

Three levels, not four, plus **Mixed** — which draws a strength *per seat*, so a
table's opponents differ from each other.

| | Easy | Medium | Hard |
|---|---|---|---|
| Avoids feeding live melds | – | ✅ | ✅ |
| Remembers the deal's discards | – | – | ✅ (all of it) |
| Tracks what opponents took off the pile | – | – | ✅ |
| Counts what is still unaccounted for | – | – | ✅ |
| Watches how close anyone is to going out | – | – | ✅ |
| Protects unfinished material | – | – | ✅ |
| Dithers about going down | 25% | 5% | – |
| Blunders a discard | 12% | 3% | – |
| Misses a lay-off | 25% | 5% | – |

Three-handed, 200 deals, Žolík Classic: **easy 20% of wins (171 mean penalty
points) · medium 30% (148) · hard 50% (119)**. Under a 35-point floor:
22% / 38% / 39%, means 162 / 146 / 124.

Medium is the agent exactly as it played before any of this, which is what makes
the other two measurable.

### 10.2 Expert was built, measured, and not shipped

A fourth level — perfect recall, outs counted against every fragment, an endgame
it reacted to a turn earlier — was written and then could not be shown to beat
Hard on any ruleset at either table size. Shipping a level someone picks
*because* it says it is harder, which is not harder, is worse than three that are
honestly ordered. Its capabilities moved into Hard, where they measure
neutral-to-positive.

The lesson: on this evidence more *perception* has run out of road. A real
fourth level has to search.

### 10.3 Ideas the sweep rejected

Each of these is in §4 above, each sounds obviously right, and each was deleted
rather than disabled — with the numbers recorded in `internal/ai/profile.go` so
the next person to have the idea gets the measurement rather than the intuition.

| Idea | Result |
|---|---|
| **Take the discard when it improves the hand's shape** (§4.6) | Lost **141 of 200** deals and ~60 penalty points a match. Taking a card commits you to it for the turn, tells the table what you are building, and buys a maybe from a pile an opponent already judged worthless. |
| **Plan the go-out** (§4.3) | Measured *exactly* neutral, twice. Shedding the most expensive card repeatedly already finds the same move. |
| **Joker economy** (§4.5) | Neutral at both table sizes. |
| **Prefer not to extend a run somebody could go out on** | Neutral. |
| **Price material in penalty points instead of ranking it above them** | Helped on top of Medium, cost 14 of 200 on top of Hard. Not robust. |

What *did* survive: protecting unfinished material (§4.1), noticing the endgame
(§4.4), and the sensors in §3 without which none of it was possible.

### 10.4 Bugs the work found

Three were pre-existing and two were mine — all five found by tests rather than
by reading:

1. **The agent could wedge its own turn.** Taking a card off the pile obliges
   melding *that card*; the agent then searched for any qualifying combination,
   laid one that did not use it, and was refused its own discard with no legal
   move left. Live bug, every difficulty, fixed in `ChooseAction`.
2. **`removeCardsOnce` panicked** when asked to remove more cards than it was
   given — a latent negative-capacity `make`.
3. **The "hard" discard-memory signal could never fire**, because the adapter
   handed the agent an empty discard history.
4. **Every unseen card counted as dead** (`copiesLeft` returned zero for a card
   absent from the map), so the counting AI dumped good material. Caught by
   `TestUnseenCountsRecoverAfterAReshuffle`.
5. **Undo un-learned a pickup** — the ledger's rewind point was taken a moment
   before the card it was supposed to remember.

### 10.5 Known gap

The strength gate asserts an ordering on both Žolíky rulesets and **reports
without asserting on Continental**, where Hard beats the others clearly on
penalty points and not at all on wins, and repeated sweeps disagree by more than
the gap. Continental's contract rotates and asks for a quota of meld *types*, so
protecting unfinished material competes with a contract that wants specific
shapes. Making `keepValue` contract-aware is the fix, and it is its own piece of
work with its own measurement.

### 10.6 Where things live

| | |
|---|---|
| The strength dial | `server/internal/ai/profile.go` |
| What the table has seen | `server/internal/ai/ledger.go`, `knowledge.go` |
| The lobby option, skills and personas | `server/internal/module/skill.go` |
| Seating a bot | `server/internal/match/handlers.go` (`addBot`) |
| Driving one | `server/internal/match/bots.go` (`botSeatFor`) |
| The simulator and the gate | `server/internal/ai/sim/` |
| The benchmark | `server/cmd/aibench` |
