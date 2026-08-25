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

  for (let i = 0; i < rects.length; i++) {
    const r = rects[i];
    if (!r) continue;
    // Inside a card is unambiguous, and beats any centre-distance comparison
    // with a neighbour that happens to be laid out closer to the pointer.
    if (point.x >= r.x && point.x <= r.x + r.width && point.y >= r.y && point.y <= r.y + r.height) {
      return i;
    }
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
  return best;
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
  return point.x < r.x + r.width / 2 ? nearest : nearest + 1;
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
