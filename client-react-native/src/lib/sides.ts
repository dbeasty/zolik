import type { Seat, Zone } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';
import { type Named, playerName } from '@/src/lib/labels';

/**
 * Who is playing with whom, once the cards are out.
 *
 * The lobby answers this before the deal — it shows the sides the module
 * worked out from the seating — and then the board used to stop saying it. A
 * player who walked into a table somebody else arranged, or who came back to
 * one after a day away, had no way to find their partner except by noticing
 * whose melds kept landing in their own spread. That is a fact about the game
 * they are in the middle of playing, so the board has to carry it too.
 *
 * Every function here reads `seat.side` and does nothing else with it. The
 * module decides who shares a side; this decides how to say so. Comparing two
 * opaque ids for equality is not a second implementation of the rule — it is
 * the only thing an opaque id is for.
 */

/**
 * Everyone sharing a seat's side, that seat excluded.
 *
 * Empty in a game where everybody plays for themselves: the server leaves
 * `side` unset there rather than inventing a side per player, so a Prší table
 * and a heads-up Canasta both come back with nothing to show.
 */
export function partnersOf(seats: Seat[], playerId: string): string[] {
  const side = seats.find((s) => s.playerId === playerId)?.side;
  if (!side) return [];
  return seats
    .filter((s) => s.side === side && s.playerId !== playerId)
    .map((s) => s.playerId);
}

/**
 * Whether a seat is on the viewer's side — their own seat excluded, because
 * "you are on your own team" is not news and the viewer's seat is already the
 * one tinted as theirs.
 */
export function isPartnerOfViewer(seats: Seat[], playerId: string, viewerId: string): boolean {
  return partnersOf(seats, viewerId).includes(playerId);
}

/**
 * A seat's partners, named.
 *
 * The viewer appears as "you" rather than by name: on a partner's tile the
 * line is there to say *you two are together*, and a player reading their own
 * name back off somebody else's tile has to do that translation themselves.
 *
 * Returns '' where the seat has no partners, which is the signal to draw
 * nothing at all — an empty line under a name reads as a missing one.
 */
export function partnerText(
  seats: Seat[],
  players: Named[],
  playerId: string,
  viewerId: string,
): string {
  const partners = partnersOf(seats, playerId);
  if (!partners.length) return '';
  return partners.map((id) => (id === viewerId ? t('match.you') : playerName(players, id))).join(', ');
}

/**
 * One area of the row of spreads: a partnership's zones together, or a zone
 * that stands on its own — a player's melds, or a side's in a game whose
 * seats carry none.
 */
export type SpreadArea = { side: string; zones: Zone[] } | { side?: undefined; zones: [Zone] };

/**
 * The row of spreads cut into areas, the viewer's own first.
 *
 * A side's zones — its melds, its red threes — are gathered wherever they
 * arrived, in the order the first of them did, because they are one answer
 * to "what has this partnership laid down?". The viewer's own area leads,
 * whether it is theirs by side or by owner, because it is the one drawn
 * directly under the hand they play into it from.
 */
export function spreadAreas(zones: Zone[], seats: Seat[], viewerId: string): SpreadArea[] {
  const mySide = seats.find((s) => s.playerId === viewerId)?.side;
  const areas: SpreadArea[] = [];
  const bySide = new Map<string, Zone[]>();
  for (const z of zones) {
    if (!z.side) {
      areas.push({ zones: [z] });
      continue;
    }
    const held = bySide.get(z.side);
    if (held) {
      held.push(z);
      continue;
    }
    const fresh = [z];
    bySide.set(z.side, fresh);
    areas.push({ side: z.side, zones: fresh });
  }
  const mine = (a: SpreadArea) =>
    a.side ? a.side === mySide : a.zones[0].ownerId === viewerId;
  return [...areas.filter(mine), ...areas.filter((a) => !mine(a))];
}

/** Everyone seated on a side, in seating order. */
export function sideMembers(seats: Seat[], side: string): string[] {
  return seats.filter((s) => s.side === side).map((s) => s.playerId);
}
