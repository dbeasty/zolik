# Why not, and what now

A plan for making every refused action explain itself, in place, in every
game — and for saying what a player *may* do, not only what they may not.

## Where this stands today

Almost all of the machinery exists. It was built for Žolíky and it works
there, end to end:

- `ActionOffer` carries `WhyNot` (a stable code), `RuleIDs` (pointers into the
  module's own written rules), `Remedy` (a sentence saying what to do instead,
  with live detail in it) and `RemedyOfferID` (the offer that performs it).
- `zolikmod/ruleindex.go` maps every code the Žolíky validators can return to
  the sentences that justify it, resolved against the house rules actually in
  force; `zolikmod/remedy.go` attaches the way out.
- `ruleindex_test.go` fails the build if a code is unmapped or an id resolves
  to a sentence this table does not state.
- The client renders the reason inline under every greyed-out control
  (`OfferBar.ReasonLine`), and `WhySheet` shows the three layers — reason,
  rule, remedy — with a working button for the remedy and a deep link into the
  rules screen scrolled to the sentence.
- A refused *drop* is a first-class thing (`DropSpot.refusal`), and a marked
  card borrows the refusal it is heading for (`refusalBehindBadge`).
- Wording lives in one locale bundle, and `serverKeys.json` fails the build for
  any emitted key nobody worded.

So this is not a green field. It is a good design that reached one game and
stopped.

## What is actually missing

**G1 — five of six games explain nothing.** Prší, Canasta, Hold'em, Rummy
Tiles and Gin Rummy all populate `WhyNot`, and none of them implements
`RuleIndexProvider` or attaches a `Remedy`. `ruleIds` and `remedy` are
therefore always empty in those games, which means the "why ›" affordance
never appears, `WhySheet` never opens, and 75 refusal codes reach the player
as one terse line with no rule behind it and no way out.

    prsi 9 · holdem 11 · ginrummy 12 · rummytiles 17 · canasta 26

**G2 — their written rules are too thin to point at.** `prsi/rules.go` states
nine sentences and can refuse nine ways; `canasta/rules.go` states rather
fewer sentences than it has codes. A rule index needs a sentence per class of
refusal, and several do not exist yet.

**G3 — nothing enforces any of it.** `allmodules_test.go` requires
`RulesProvider` and never asks for `RuleIndexProvider`; the coverage test
lives *inside* `zolikmod`. Game seven ships mute by default and no test
notices.

**G4 — the explanation is a modal.** The reason is inline, which is right, but
the rule and the remedy require a bottom sheet that covers the board — so the
player loses sight of the thing they were trying to do at the exact moment
they are being told about it. A refused drop does the same: it opens a sheet
rather than saying anything where the card was let go.

**G5 — a refused *composition* is explained by the client, badly.** The hint
under a composite meld control comes from `drops.fits()`, which knows arity
and nothing else: "pick 3+ cards", "you picked two, this takes one". The
server knows the real answer — a run must be one suit, that rank is already
melded, you are eleven points short — and is never asked until the move is
submitted. This is the case the request names, and it is the one place the
rule-derived-facts-come-from-the-server discipline is currently broken by
omission.

**G6 — the terminal client prints raw codes.** `client-tui/ui/match.go:452`
renders `o.WhyNot` verbatim, so a player sees `DISCARD_CARD_NOT_MELDED`. Its
`api.ActionOffer` does not decode `ruleIds`, `remedy` or `remedyOfferId` at
all.

**G7 — outside the match there is no offer protocol.** Lobby *Start*, *Add a
bot* and *Invite* are always enabled and fail afterwards into a plain error
string. "Greyed out, and here is why" stops at the table edge.

**G8 — everything is refusal-shaped.** Nothing tells a player what the game
expects of them now, what they *can* do, or what a rule means before they
break it. `ViewModel.Prompts` exists and is rendered as a muted line.

## The plan

Six phases. Phase 0 is the forcing function and should land first; phases 1
and 2 are independent of each other and can go in parallel.

### Phase 0 — one contract, six games

Move the coverage test out of `zolikmod` and into the shared conformance
harness, generalised:

1. Every registered module implements `module.RuleIndexProvider`.
2. Every refusal code a module can emit maps to at least one rule id.
   Codes are discovered by AST scan over each module's package for
   `Err… = "CODE"` string constants — the same trick `emittable.go` already
   uses for label keys, so a new code cannot hide.
3. Every id returned resolves to a sentence that module's own `Rules()`
   builds — for **every** declared variation crossed with every declared
   option value, not just the defaults, because half of Žolíky's mappings are
   already config-sensitive.
4. Every code has wording (`err.<CODE>`) in the bundle — extend the existing
   `serverKeys.json` gate to the refusal vocabulary rather than only to label
   keys.

Deliberately *not* asserted: that a remedy exists. Some refusals genuinely
have none ("it is not your turn"), and inventing one is worse than none.

This phase fails the build for the five mute games. That is the point.

### Phase 1 — write the five indexes and the five remedy tables

Per module, three files following the Žolíky shape exactly:

- `rules.go` — expand until every refusal class has a sentence to point at.
- `ruleindex.go` — `ExplainRefusal(cfg, code) []string`, sensitive to the
  options in force where the rule is.
- `remedy.go` — `annotate(state, playerID, offers)`, attaching `Remedy` and,
  where the way out is on the offer list, `RemedyOfferID`.

Order by how much confusion each buys, most first:

| # | Module | Codes | Why this order |
|---|--------|-------|----------------|
| 1 | Canasta | 26 | The most rules per square inch: frozen piles, black threes, rank-already-melded, natural minimums. Every one of those is currently a capitalised code. |
| 2 | Rummy Tiles | 17 | A turn that rearranges the table is the hardest thing in the app to be refused by, and `ErrTileDoesNotFit` says nothing about which end. |
| 3 | Gin Rummy | 12 | Knocking has a deadwood threshold the player cannot see; the remedy is arithmetic the server already did. |
| 4 | Hold'em | 11 | Fewest rules, but a min-raise refusal is pure numbers, so the remedy is exact and cheap. |
| 5 | Prší | 9 | Simplest game, mildest pain — but also the cheapest to finish. |

New keys go through the `serverKeys.json` regeneration in the same commit, or
CI fails.

### Phase 2 — explain in place

The change the request is actually about. Three surfaces, one component:

**2.1 Inline disclosure under the control.** Pressing "why ›" expands the
explanation *under the button that is greyed out* — reason, rule, remedy, and
the remedy button — rather than raising a sheet over the board. Nothing else
moves; the disclosure pushes the offer row down and pushes it back on close.
`WhySheet` becomes `WhyBlock`, a plain view, and the modal wrapper survives
only as the narrow-phone fallback where there is no room to expand.

**2.2 A callout where the card was let go.** A refused drop draws its reason
anchored at the meld or zone that refused it, for as long as the player holds
the card there, and stays as a dismissible callout on release. The board keeps
its own picture; the sentence arrives next to the thing it is about.

**2.3 The meld that will not take the card says so.** Group-level refusals
(`ErrCardDoesNotExtendMeld`, wrong run end, rank already melded) render on the
group, not in the offer bar, using the badge machinery `Group.BadgeKeys`
already provides.

### Phase 3 — explain a composition *before* it is submitted

A dry run, so the server can answer "is this meld legal, and if not why" while
the player is still choosing cards.

- **Wire.** The match socket currently unmarshals a bare `module.Action`. Add
  an envelope: `{"type":"probe","action":{…}}`; a frame with no `type` is
  still an action, so nothing existing changes. The reply is
  `{"type":"probeResult","offerId":…,"ok":false,"code":…,"ruleIds":[…],"remedy":…}`.
- **Server.** `Apply` is already pure and every module already probes itself
  this way to build its offer list. The handler runs it against a copy,
  discards the state, and returns the refusal annotated exactly as an offer
  would have been. It writes nothing and takes no lock beyond the read.
- **Client.** Composite selection changes fire a debounced probe (150 ms, only
  once the selection meets `minCards`). The hint under the meld control stops
  being client arithmetic and becomes the engine's own sentence.

Cost control: one in-flight probe per player, dropped if superseded; probes
are rate-limited alongside actions and never persisted.

### Phase 4 — help, not only refusal

1. **A coach line.** `ViewModel.Prompts` is already the module saying what it
   is waiting for. Give it a real slot near the board rather than a muted line
   under everything, and make it the resting state of the explanation area, so
   the space does not appear and disappear.
2. **"What can I do?"** A panel listing the enabled offers with their facts
   and, next to each, the rule that governs it — the mirror image of the
   refusal path, built from the same three fields.
3. **A help button in the match header** opening this table's own rules — the
   screen and the deep link already exist and are reachable only from a
   refusal today.
4. **First-match coaching.** Per game, dismissible, keyed off "this account
   has never finished a match of this module" (the stats service already knows).

### Phase 5 — the lobby

Give the table screen the same shape the board has: the lobby state carries
its own `preconditions` — id, enabled, `whyNot`, `remedy` — so *Start* is
greyed out with "you need one more player" and an *Add a bot* remedy button,
rather than being pressable and failing afterwards. Same three fields, no new
vocabulary.

### Phase 6 — the terminal client

Decode `ruleIds`, `remedy` and `remedyOfferId`, show the remedy under the
selected offer, and bind `?` to the rule behind it.

**Open question for approval:** the TUI deliberately ships no locale bundle —
`labelOf` degrades an unknown key to its last segment — so today it can render
neither `err.CODE` wording nor a rule sentence. Either it embeds the bundle
(the honest fix, and it is one JSON file), or the rules endpoint grows a
`?lang=` that returns resolved text for clients that have no bundle (simpler,
but puts rendered sentences on the wire, which the protocol has avoided
everywhere else on purpose). Recommendation: embed the bundle.

## What stays true throughout

The rule the design already enforces, unchanged:

> Every rule-derived fact is computed once, on the server, inside the module —
> and shipped. A client that has to *derive* a rule is a bug.

Phase 3 exists because `drops.fits()` is currently a small violation of it,
and phases 0 and 1 exist because five games are shipping the fact and no
explanation.
