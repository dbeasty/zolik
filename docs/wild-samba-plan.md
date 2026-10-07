# CanastaX — plan

Status: built on `claude/canastax` (2026-10-07). Server rules, client
interaction and heuristic-bot support are in; trained models read CanastaX
as Samba until one is trained on it.

A house-rules Samba where the table is alive: your side reshapes its own melds,
sequences may carry wilds, a meld of 2s is the big prize, and any wild on the
table — yours or theirs — can be bought out with the card it stands for.

## 1. Rules

Everything below is Samba (three decks, 15 cards, draw two, pile always frozen,
two canastas to go out, 10 000 to win) except where it says otherwise.

### 1.1 Dirty sequences
- A sequence (3–7 cards, one suit, consecutive) may contain wilds, under the
  same limits as a Samba group: at most **2 wilds**, and at least **twice as
  many naturals as wilds** (one wild needs 2 naturals, two wilds need 4).
- Every wild in a sequence **stands for one position** (in 5♥ 🃏 7♥ the joker
  is the 6♥). The player says which when laying it; a wild at the end of a run
  may be the card above or below.
- Seven cards close it, as now:
  - clean (no wilds) — a **samba**, 1500;
  - dirty — a **dirty samba**, **700**.

### 1.2 The 2s meld
- A meld made of 2s, one per side. It may also hold jokers, under the group
  limits: at most 2 jokers, at least twice as many 2s as jokers.
- Seven cards is a canasta and counts toward the two needed to go out:
  - all 2s — **2250** (Samba's 1500 + 50%);
  - with jokers — **1500**.
- Not part of an initial meld: three 2s are 60 points, which would make opening
  trivial. Laid only once the side has opened.

### 1.3 Rearranging your own melds
- On your turn, after drawing and before discarding, once your side has opened.
- Move one or more cards from one of your side's melds to another. Wilds move
  too (and a wild moved into a sequence takes a position, as when laying).
- **Every move must leave both melds legal.** A meld may be emptied entirely
  (it disappears). Canastas are not protected: a canasta that drops below seven
  is no longer a canasta.
- No hand cards are involved, so a move never changes what you hold.

### 1.4 Poaching wilds
- On your turn, after drawing and before discarding, once your side has opened.
- Any wild on the table — any side's meld, canasta or not — can be replaced by a
  natural from your hand that it stands for:
  - in a group: a natural of the group's rank;
  - in a sequence: the exact card for the wild's position;
  - in the 2s meld: a joker is replaced by a 2 (a 2 there is not poachable — it
    is already the natural card of that meld).
- The wild goes **into your hand**, to use whenever you like.
- Side effect, by design: taking the last wild out of an opponent's mixed
  canasta makes it natural (300 → 500; a dirty samba becomes a samba, 700 →
  1500). Poaching costs them a wild and may pay them for it.

### 1.5 Top card or whole pile
- Wherever a capture of the discard pile is legal, the same capture may take
  the top card alone: melded with the same cards from hand (or onto the same
  meld), and the rest of the pile stays where it is, frozen or not as it was.
  Like a capture, it replaces the draw. Verb `take_pile_top`.

### 1.6 Guards that already exist and keep holding
- A move or poach that would leave the turn unfinishable (e.g. breaking the
  canasta you need to go out while holding one card) is refused, the same way a
  lay is today (`checkTurnFinishes`).
- Moves and poaches are undoable within the turn, in strict last-in-first-out
  order with the existing lay/lay-off/capture undos.

## 2. Packaging

Five table options on Canasta, each off by default so no existing table changes
(house rules are options, not constants):

| Option | Values | Notes |
|---|---|---|
| `dirtySequences` | off / on | needs a variation with sequences |
| `wildMeld` | off / on | the 2s meld |
| `rearrange` | off / on | |
| `poach` | off / on | most useful with `dirtySequences` |
| `topOnlyCapture` | off / on | |

Plus a preset variation, **CanastaX** (`canastax`), = Samba with all five on.
Its trained-model encoding reads as Samba (`learn.go`'s `variationName`) until a
model is trained on it.

## 3. Build order

Each phase ships on its own and is playable.

**A. Server rules (canasta module)**
1. Ruleset fields + options + preset; written `Rules()` sections per option
   value (the rules screen renders them); refusal codes with rule ids.
2. Positional wilds in sequences: storage (a wild records the card it stands
   for), validation, lay/lay-off with a chosen position, scoring (dirty samba).
   This is the largest change: Samba's run offers, `take_top`,
   `reachableValue` and the dead-end search all assume runs hold no wilds.
3. 2s meld: kind `wild`, validation, scoring, round-result lines.
4. `move_cards` verb + `undo_move`.
5. `poach` verb + `undo_poach`.
6. Tests: unit per rule, plus the existing dead-end sweeps run over the preset
   with several seeds (wedges hide in Samba).

**B. Client** — the table is currently read-only; cards inside a meld cannot
be selected or dragged (only hand cards can).
1. Selecting cards inside your own melds; drag/tap them onto another of your
   melds (`move_cards`).
2. Selecting a wild in any meld + the matching card in hand (`poach`).
3. Choosing a wild's position when a dirty sequence is ambiguous.
4. Labels in all locales; `serverKeys.json`.

**C. Bots**
1. Heuristic bot: lay dirty sequences and the 2s meld; poach when it completes
   or upgrades one of its own canastas, or strips an opponent of a wild they
   need; ignore rearranging at first.
2. Trained models: later, once there are games to learn from.

## 4. As built

- Engine: `server/internal/canasta/house.go` (run positions, the 2s meld,
  `move_cards`, `poach`, `undo_reshape`, the offers for them); a dirty run
  stores its start in `Meld.Low` and its cards in position order.
- Undo: lays, moves and poaches unwind last-in-first-out across stacks
  (`Seq`); an undo out of order is refused with `UNDO_LATEST_FIRST`.
- A wild that could join a dirty run at either end carries `front`/`end`
  placements; where it is let go on the run decides which end.
- Client: melds whose cards can move are tapped open, then their cards picked
  and a lit meld tapped; poaching is a hand card dropped/tapped onto any meld
  with a wild it stands for. Moves are not in the button bar.
- Bots: poach off opponents (never the last wild of a canasta, which would
  pay them), move a card only to finish a canasta without breaking one, and
  lay dirty runs and the 2s meld through the ordinary meld offers.
- Decided in chat: a 2s canasta with jokers pays 1500; a poached wild may be
  laid the same turn.
- Not yet: choosing which end a wild goes on when laying a *new* dirty run
  (it goes on top unless the top is the ace); a trained model for CanastaX.
