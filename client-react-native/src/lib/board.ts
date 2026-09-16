import type { Zone } from '@/src/api/matchTypes';
import { zoneElementId } from '@/src/lib/drops';

/**
 * Which zones are worth putting on screen.
 *
 * A hidden zone arrives with a *count and no cards* — the module's whole
 * anti-cheat surface, not a client concern. Drawing a panel for one anyway
 * says nothing a player didn't already know from their seat's own card
 * count, and costs a full-width row saying so. So it isn't drawn, unless
 * it's the one thing that would make hiding it a bug: the viewer's own zone,
 * or a target the card in flight could land on.
 *
 * A zone that is simply *empty* — count 0, nothing concealed — is a
 * different thing entirely and is never hidden by this: an empty spread is
 * where the game's first meld lands, owned or not (a Canasta team's melds
 * belong to no single player), and disappearing until the first card lands
 * in it is the exact bug `zolikmod`'s view already works around for the
 * viewer's own spread (see its own comment). Concealment is a property of
 * `count > 0` with nothing shown for it; emptiness on its own conceals
 * nothing.
 */

/**
 * How many cards a zone holds that it is not showing this viewer.
 *
 * The server withholds a card by *not sending it* and letting the count say
 * so — that is the whole anti-cheat surface. Which leaves the client holding
 * a fact it was not drawing: a blackjack dealer arrives as "two cards, here
 * is one of them", and the board drew the upcard and then nothing at all,
 * with a gap on the felt where every real table has a card lying face down.
 *
 * So this is the number of backs to draw. Counting *both* loose cards and the
 * cards inside groups, because a spread keeps its cards in its groups and
 * subtracting only the loose ones would have every meld on the board grow a
 * row of phantom face-down cards.
 *
 * A stack is excluded: it is about its count by design and is already drawn
 * as a deck of backs (`StackBack`), so counting it here would draw the pile
 * twice.
 */
export function concealedCount(zone: Zone): number {
  if (zone.kind === 'stack') return 0;
  const inGroups = (zone.groups ?? []).reduce((n, g) => n + g.cards.length, 0);
  const shown = (zone.cards ?? []).length + inGroups;
  return Math.max(0, zone.count - shown);
}

/** A zone claiming to hold cards it isn't showing this viewer. */
export function isConcealed(zone: Zone): boolean {
  if (zone.kind === 'stack') return false; // about its count by design — StackBack draws it.
  if ((zone.cards ?? []).length > 0) return false;
  if ((zone.groups ?? []).length > 0) return false;
  return zone.count > 0;
}

/**
 * The zones worth drawing for this viewer: their own always, anything not
 * concealed, and anything the card currently in flight could be dropped on.
 * What's left is a count with nothing shown for it and nothing live about
 * it — exactly what the seat strip already reports.
 */
export function drawableZones(
  zones: Zone[],
  viewerId: string,
  activeDrops: ReadonlySet<string>,
): Zone[] {
  return zones.filter((zone) => {
    if (zone.ownerId === viewerId) return true;
    if (activeDrops.has(zoneElementId(zone.id))) return true;
    return !isConcealed(zone);
  });
}
