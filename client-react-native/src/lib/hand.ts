/**
 * A hand as *slots* rather than as card strings.
 *
 * The shell addresses cards by their string ("AS", "7H") everywhere else, and
 * for a single-deck game that is enough: a card string names exactly one card.
 * Two of this repo's four games do not have that property. Canasta is fixed at
 * two decks plus four jokers, and Žolíky scales decks with the table and never
 * uses fewer than two, so a hand holding two identical strings is normal in
 * both.
 *
 * The server has always modelled this correctly — `canasta.removeCards` takes
 * "one copy per request, not all matching copies, because with two decks a
 * player can hold two identical cards and mean only one of them". The client
 * did not: it selected by string, so tapping one of a pair lit up both, and
 * tapping the second cleared the pair. This file is the client side of that
 * same distinction.
 *
 * A slot is one physical card in one position, with an identity that outlives
 * the state pushes the server sends after every move. That identity is what
 * lets a player point at a particular copy — to select it, or to drag it
 * somewhere else in the fan.
 *
 * Nothing here knows a rule. Arrangement is a view preference, not a fact
 * about the game, which is why it lives on this side of the wire at all: the
 * order cards are held in changes nothing about which of them may be played,
 * and a submission still travels as card strings.
 */

import { cardSuit, displayRank } from '@/src/lib/cards';

export type Slot = {
  /** Stable for as long as this physical card stays in the hand. */
  id: string;
  card: string;
};

export type Rect = { x: number; y: number; width: number; height: number };

/**
 * Reconciles a player's arrangement with the hand the server just sent.
 *
 * Cards still held keep their slot — and therefore their position and their
 * selected-ness — in the order the player put them in. Cards that arrived
 * since the last push are appended in the order the server sent them, because
 * a drawn card turning up at the end of the fan is predictable, whereas
 * inserting it by rank would silently undo an arrangement someone had just
 * made by hand.
 *
 * Matching is by multiset, so a pair of identical cards keeps two slots and
 * playing one of them retires exactly one.
 */
export function arrangeSlots(
  previous: Slot[],
  incoming: string[],
  mint: (card: string) => string,
): Slot[] {
  const unplaced = new Map<string, number>();
  for (const card of incoming) unplaced.set(card, (unplaced.get(card) ?? 0) + 1);

  const kept: Slot[] = [];
  for (const slot of previous) {
    const left = unplaced.get(slot.card) ?? 0;
    if (left > 0) {
      kept.push(slot);
      unplaced.set(slot.card, left - 1);
    }
  }

  const gained: Slot[] = [];
  for (const card of incoming) {
    const left = unplaced.get(card) ?? 0;
    if (left > 0) {
      gained.push({ id: mint(card), card });
      unplaced.set(card, left - 1);
    }
  }

  return [...kept, ...gained];
}

/**
 * Which slots in `next` were not in `previous` — but only when nothing left
 * the hand in the same beat.
 *
 * A card arriving alone, with everything already held still there, is the
 * shape of picking one up: the moment a player most wants to see which card
 * that was, since it's the one thing in the fan they didn't have a second
 * ago. A card arriving *alongside* others leaving — the whole hand turning
 * over between deals — looks like the same "new id in the fan" from this
 * function's point of view, so it is deliberately excluded rather than
 * guessed at: nothing here says which zone this is or when a deal happens.
 *
 * An empty `previous` returns nothing for the same reason: the very first
 * hand a player is dealt is every one of its cards "arriving" at once, and
 * none of them is a card anyone picked up.
 */
export function justArrived(previous: Slot[], next: Slot[]): string[] {
  if (previous.length === 0) return [];
  const previousIds = new Set(previous.map((s) => s.id));
  const gained = next.filter((s) => !previousIds.has(s.id));
  if (gained.length === 0) return [];
  if (next.length - gained.length !== previous.length) return [];
  return gained.map((s) => s.id);
}

/**
 * Moves one item to another position, returning a new array.
 *
 * `to` is the index the item should end up at once it has been lifted out,
 * which is how a drop between two cards reads: dragging the first card onto
 * the third lands it third, not second.
 */
export function moveSlot<T>(items: T[], from: number, to: number): T[] {
  if (from < 0 || from >= items.length) return items;
  const target = Math.max(0, Math.min(to, items.length - 1));
  if (from === target) return items;

  const out = items.slice();
  const [moved] = out.splice(from, 1);
  out.splice(target, 0, moved);
  return out;
}

/** Ace-low rank order, the one thing a shell may know about a card without
 * knowing a rule: it's a fact about the deck, not about any game's play. */
function rankOrder(card: string): number {
  if (card.startsWith('JOKER')) return 100;
  const order: Record<string, number> = {
    A: 1,
    '2': 2,
    '3': 3,
    '4': 4,
    '5': 5,
    '6': 6,
    '7': 7,
    '8': 8,
    '9': 9,
    '10': 10,
    J: 11,
    Q: 12,
    K: 13,
  };
  return order[displayRank(card)] ?? 50;
}

/**
 * One-tap tidying: low to high, with likely meld material held together.
 *
 * Same-rank duplicates (set material) cluster together, same-suit
 * consecutive-rank cards (run material) cluster together, and each cluster is
 * placed by its lowest rank — so a hand a player has not touched yet already
 * reads as "here is what you could lay" rather than as an arbitrary sort.
 * Jokers are wild and don't belong to any one cluster, so they sit together
 * at the end.
 *
 * A permutation of the slots handed in, never a new one: the point of a slot
 * is that it survives being moved, and an auto-arrange a player doesn't like
 * should undo the same way a manual drag does.
 */
export function arrangeAuto(slots: Slot[]): Slot[] {
  const jokers = slots.filter((s) => s.card.startsWith('JOKER'));
  const nonJokers = slots.filter((s) => !s.card.startsWith('JOKER'));

  const rankCounts = new Map<string, number>();
  for (const s of nonJokers) {
    const r = displayRank(s.card);
    rankCounts.set(r, (rankCounts.get(r) ?? 0) + 1);
  }

  const multiples: Slot[] = [];
  const singles: Slot[] = [];
  for (const s of nonJokers) {
    if ((rankCounts.get(displayRank(s.card)) ?? 0) >= 2) multiples.push(s);
    else singles.push(s);
  }

  const multipleGroups = new Map<string, Slot[]>();
  for (const s of multiples) {
    const r = displayRank(s.card);
    const arr = multipleGroups.get(r) ?? [];
    arr.push(s);
    multipleGroups.set(r, arr);
  }
  for (const arr of multipleGroups.values()) {
    arr.sort((a, b) => cardSuit(a.card).localeCompare(cardSuit(b.card)));
  }

  const bySuit = new Map<string, Slot[]>();
  for (const s of singles) {
    const suit = cardSuit(s.card);
    const arr = bySuit.get(suit) ?? [];
    arr.push(s);
    bySuit.set(suit, arr);
  }

  const groups: { key: number; slots: Slot[] }[] = [];

  for (const arr of bySuit.values()) {
    arr.sort((a, b) => rankOrder(a.card) - rankOrder(b.card));
    let run: Slot[] = [];
    const flush = () => {
      if (run.length === 0) return;
      groups.push({ key: rankOrder(run[0].card), slots: run });
      run = [];
    };
    for (const s of arr) {
      if (run.length === 0 || rankOrder(s.card) === rankOrder(run[run.length - 1].card) + 1) {
        run.push(s);
      } else {
        flush();
        run.push(s);
      }
    }
    flush();
  }

  for (const arr of multipleGroups.values()) {
    groups.push({ key: rankOrder(arr[0].card), slots: arr });
  }

  groups.sort((a, b) => a.key - b.key);

  return [...groups.flatMap((g) => g.slots), ...jokers];
}

/**
 * The slot nearest a pointer, given where each one was laid out.
 *
 * Measured rather than calculated from a card width, because the fan wraps: a
 * hand of fourteen is several rows on a phone, and an index derived from
 * horizontal offset alone would put a card dropped on the second row into the
 * first.
 *
 * Rectangles and point only have to agree with each other; which space they
 * are in is the caller's business.
 *
 * Returns null when nothing has been measured yet, which is the honest answer
 * during the first frame of a drag.
 */
export function slotAtPoint(
  rects: (Rect | undefined)[],
  point: { x: number; y: number },
): number | null {
  let best: number | null = null;
  let bestDistance = Infinity;
  // The topmost box the point is inside, if it is inside any.
  let inside: number | null = null;

  for (let i = 0; i < rects.length; i++) {
    const r = rects[i];
    if (!r) continue;
    // Inside a card beats any centre-distance comparison with a neighbour
    // that happens to be laid out closer to the pointer.
    //
    // *Which* card, when several contain the point, is the whole subtlety
    // here: a hand long enough to be fanned has every card overlapping the
    // one before it, so most points are inside three or four boxes at once.
    // The answer is the last one — the cards are drawn in order and each is
    // painted over the one before it, so the topmost box is the card the
    // player can actually see under the pointer, and picking the first
    // instead means dragging to the right and being told you are still over
    // a card several places to the left.
    if (point.x >= r.x && point.x <= r.x + r.width && point.y >= r.y && point.y <= r.y + r.height) {
      inside = i;
      continue;
    }
    if (inside !== null) continue;
    const dx = point.x - (r.x + r.width / 2);
    const dy = point.y - (r.y + r.height / 2);
    // Vertical distance is weighted, so a pointer below the fan prefers the
    // nearest card on the row it is actually over rather than a horizontally
    // closer one a row above.
    const distance = dx * dx + dy * dy * 4;
    if (distance < bestDistance) {
      bestDistance = distance;
      best = i;
    }
  }
  return inside ?? best;
}

/**
 * The cards a drag starting on one slot carries.
 *
 * Dragging a card that is already selected takes the whole selection with it,
 * which is how three cards become one meld in a single gesture. Dragging an
 * unselected card takes only that card — and deliberately does not clear the
 * selection, since a drag that is refused should leave the table as it found
 * it. The rule matters because the alternative, "always drag the selection",
 * makes a selection left over from an abandoned move ride silently along with
 * the next card anyone picks up.
 */
export function slotsForDrag(
  slots: Slot[],
  selected: ReadonlySet<string>,
  index: number,
): Slot[] {
  const picked = slots[index];
  if (!picked) return [];
  if (!selected.has(picked.id)) return [picked];
  return slots.filter((s) => selected.has(s.id));
}

/**
 * Puts slots back into an order recorded earlier.
 *
 * Used when a hand comes back — a reload, or returning to a match — where the
 * cards are the same cards but the slots holding them are freshly minted, so
 * the arrangement can only be remembered as card names. Matching is by
 * multiset, like everything else here, so a remembered pair is still a pair.
 *
 * Anything the record does not account for keeps its place at the end rather
 * than being dropped: a hand that has moved on since the order was written is
 * still mostly in the order its owner left it.
 */
export function applySavedOrder(slots: Slot[], saved: string[]): Slot[] {
  const byCard = new Map<string, Slot[]>();
  for (const slot of slots) {
    const held = byCard.get(slot.card);
    if (held) held.push(slot);
    else byCard.set(slot.card, [slot]);
  }

  const placed: Slot[] = [];
  const used = new Set<string>();
  for (const card of saved) {
    const next = byCard.get(card)?.shift();
    if (next) {
      placed.push(next);
      used.add(next.id);
    }
  }
  for (const slot of slots) if (!used.has(slot.id)) placed.push(slot);
  return placed;
}

/**
 * Which *gap* between cards a pointer is nearest — 0 to n, not 0 to n-1.
 *
 * A card does not land "on" another card, it lands between two of them, and
 * there is one more gap than there are cards. That extra one matters: without
 * it there is no way to say "after the last card", so the right-hand end of
 * the fan is unreachable by dragging.
 *
 * Which side of a card a gap is on comes from which half of it the pointer is
 * in, so aiming just left of a card means "before this one" — the same way
 * dropping into a list of anything else behaves.
 */
export function insertionAtPoint(
  rects: (Rect | undefined)[],
  point: { x: number; y: number },
): number | null {
  const nearest = slotAtPoint(rects, point);
  if (nearest === null) return null;
  const r = rects[nearest];
  if (!r) return null;
  // Halfway across the part of this card the player can see, which in a fanned
  // hand is the strip before the next card covers it rather than the whole
  // box. Using the full width would put the tipping point under the card two
  // places along, so the gap would open a card late all the way down the fan.
  const next = rects[nearest + 1];
  const visible =
    next && next.y === r.y && next.x > r.x ? Math.min(r.width, next.x - r.x) : r.width;
  return point.x < r.x + visible / 2 ? nearest : nearest + 1;
}

/**
 * The index `moveSlot` needs to land a card in a given gap.
 *
 * The two count differently and it is worth being explicit about why: a gap is
 * a position in the row *as it is now*, while `moveSlot`'s index is where the
 * card ends up *after it has been lifted out*. Everything to the right of where
 * it came from has closed up by one by then.
 *
 * The two gaps either side of the card being dragged both mean "leave it where
 * it is", which is what makes letting go mid-wobble a no-op rather than a
 * one-place nudge.
 */
export function moveTargetFor(from: number, insertion: number): number {
  return insertion > from ? insertion - 1 : insertion;
}

/** How a fan is drawn while it is being pulled apart to take a card. */
export type FanSplit = {
  /**
   * How far each card is drawn from where it was laid out, by card index.
   * Zero for the card being carried, and for every card on a row the hole is
   * not on.
   */
  shift: number[];
  /**
   * How far the hole itself is drawn from the column it sits in. It travels
   * with the cards on its own left, so the space that opens is one clear
   * opening rather than two half ones either side of it.
   */
  gap: number;
  /**
   * How much clear space actually opened. Short of `want` only when the fan
   * had nothing left to give, which is worth knowing rather than discovering
   * by eye.
   */
  opening: number;
};

/**
 * How far the hand comes apart to show where a card would land.
 *
 * A closed hand has no room in it. The hole standing in for the dragged card
 * is a whole card wide, but it sits on the same pitch as everything else, so
 * all of it but the strip a card peeks by is covered by the card after it. So
 * the hand opens: the cards either side move out of the way until there is a
 * card-shaped hole exactly where the card will go.
 *
 * Room is found in this order, and the order is the design:
 *
 *  1. **The empty felt right of the last card.** Free, and it leaves the hole
 *     exactly where the pointer put it, which matters — the hole is the answer
 *     to "where does this land?" and an answer that slides away from the
 *     finger while you watch is a worse answer.
 *  2. **Closing up the cards right of the hole**, evenly, down to `floor`.
 *  3. **The empty felt left of the first card**, which on a hand that starts
 *     flush with the row is nothing at all.
 *  4. **Closing up the cards left of the hole.**
 *
 * Steps 2 and 4 are what make this work on the screens where it never did.
 * A closed hand sits exactly on the pitch it can still be read at and fills
 * its row to the edge — that is what `scaleFor` solves for — so on those
 * screens steps 1 and 3 between them find nothing, and what used to happen
 * was a hole the width of a card's peeking strip. `floor` is a second, lower
 * pitch that only applies while a card is out of the hand (`dragPeek`): a fan
 * with a card lifted out of it is being aimed at rather than read, and it
 * springs back the moment the card is let go.
 *
 * Everything here is moved by `transform` and nothing by layout. The row keeps
 * the shape it had when the drag started, so positions measured once, at
 * pick-up, stay true — which is what lets the hole be a *picture* of the
 * insertion point rather than its source. `slotAtPoint` still answers against
 * the resting fan. One honest consequence of that: while the fan is tightened
 * the hole travels through it slightly faster than the cards it passes, since
 * a step of the pointer is still a whole resting pitch. The hole stays under
 * the pointer, which is the only thing that was ever promised.
 *
 * Rectangles, `row`, `pitch`, `want` and `floor` only have to agree with each
 * other; which space they are in, and where the numbers came from, is the
 * caller's business.
 */
export function splitFan(opts: {
  /** Where every card rested, measured before any of them was lifted. */
  rects: (Rect | undefined)[];
  /** The row they rest in, which is what says where the empty felt is. */
  row: Rect | null;
  /** The card that has left the row and is being carried. */
  held: number;
  /** The gap the hole is open at, counted the way `insertionAtPoint` counts. */
  insertion: number;
  /** How far apart the cards are laid out. */
  pitch: number;
  /** How much wider than `pitch` a whole card is — nothing, in a laid-out hand. */
  want: number;
  /** The least a card may be drawn from the next while one is being carried. */
  floor: number;
}): FanSplit {
  const { rects, row, held, insertion, pitch, want, floor } = opts;
  const n = rects.length;
  const none: FanSplit = { shift: new Array(n).fill(0), gap: 0, opening: 0 };

  if (!row || want <= 0 || n === 0) return { ...none, opening: Math.max(0, want) };
  if (held < 0 || held >= n || insertion < 0 || insertion > n) return none;
  // Before everything has been measured there is nothing to say, which is the
  // honest answer during the first frame of a drag.
  const columns: Rect[] = [];
  for (const r of rects) {
    if (!r) return none;
    columns.push(r);
  }

  // Which column each box is drawn in. The row holds exactly the boxes it held
  // at rest — the carried card left the flow and the hole joined it — so the
  // columns *are* the places the cards were measured in, and this is the whole
  // of the arithmetic: a card loses a place if the carried card was before it,
  // and gains one if the hole is.
  const columnOf = (i: number) => i - (held < i ? 1 : 0) + (i >= insertion ? 1 : 0);
  const kGap = insertion - (held < insertion ? 1 : 0);

  // The run of columns on the hole's own row. A wrapped hand opens on one row
  // and the others are none of its business — measuring the empty felt across
  // all of them at once finds the end of the *last* row, which on a full first
  // row is how a hole came to have no room at all.
  const bandY = columns[kGap].y;
  let first = kGap;
  while (first > 0 && columns[first - 1].y === bandY) first -= 1;
  let last = kGap;
  while (last < n - 1 && columns[last + 1].y === bandY) last += 1;

  // Nothing is covering the hole, so nothing has to move for it to be seen.
  if (last === kGap) return { ...none, opening: want };

  const rightOf = last - kGap;
  const leftOf = kGap - first;
  const roomAfter = Math.max(0, row.x + row.width - (columns[last].x + columns[last].width));
  const roomBefore = Math.max(0, columns[first].x - row.x);
  const squeeze = Math.max(0, pitch - floor);

  let budget = want;
  const pushRight = Math.min(budget, roomAfter);
  budget -= pushRight;
  const stepRight = rightOf > 1 ? Math.min(squeeze, budget / (rightOf - 1)) : 0;
  budget -= stepRight * (rightOf - 1);
  const pullLeft = Math.min(budget, roomBefore);
  budget -= pullLeft;
  const stepLeft = leftOf > 1 ? Math.min(squeeze, budget / (leftOf - 1)) : 0;
  if (leftOf > 1) budget -= stepLeft * (leftOf - 1);

  // The outermost card on each side moves by the slack and no further; every
  // step back towards the hole gains one more squeeze, so the card next to the
  // hole has moved the whole way. Rounded here, at the end — rounding the step
  // instead lets the error stack up a pixel per card and the hole comes out
  // the wrong size.
  const gap = -Math.round(pullLeft + Math.max(0, leftOf - 1) * stepLeft) || 0;
  const shift = new Array<number>(n).fill(0);
  for (let i = 0; i < n; i++) {
    if (i === held) continue;
    const k = columnOf(i);
    if (k < first || k > last) continue;
    if (k > kGap) shift[i] = Math.round(pushRight + (last - k) * stepRight);
    else if (k < kGap) shift[i] = -Math.round(pullLeft + (k - first) * stepLeft);
  }

  return { shift, gap, opening: Math.round(want - budget) };
}

/** The cards a set of selected slots stands for, in the order they are held. */
export function cardsForSelection(slots: Slot[], selected: ReadonlySet<string>): string[] {
  return slots.filter((s) => selected.has(s.id)).map((s) => s.card);
}

/** Drops selections whose slot has left the hand, so nothing stale survives. */
export function pruneSelection(slots: Slot[], selected: ReadonlySet<string>): Set<string> {
  const live = new Set<string>();
  for (const s of slots) if (selected.has(s.id)) live.add(s.id);
  return live;
}

/**
 * What tapping one card does to the selection.
 *
 * Normally a toggle: taps accumulate, because gathering several cards is how
 * a meld gets built.
 *
 * The exception is a `provisional` selection — one the *app* made rather than
 * the player, which today means the card that just arrived in hand landing
 * ready to play. Touching a *different* card then replaces it rather than
 * joining it. Without that, the commonest turn there is goes wrong: the drawn
 * card is already picked, the player taps the card they actually mean to
 * play, and now two are picked — so an offer that takes exactly one (a
 * discard, a lay-off) matches neither, nothing lights up, and nothing says
 * the cure is to go and unpick a card they never picked.
 *
 * Tapping the provisional card itself still just clears it. That is a player
 * saying "not that one", and it should mean what it says rather than being
 * swallowed as "start again from that one".
 */
export function toggleSelection(
  slots: Slot[],
  selected: ReadonlySet<string>,
  slotId: string,
  opts: { provisional?: boolean } = {},
): Set<string> {
  const next = pruneSelection(slots, selected);
  if (opts.provisional && !next.has(slotId)) return new Set([slotId]);
  if (next.has(slotId)) next.delete(slotId);
  else next.add(slotId);
  return next;
}
