import type { CardView, Zone } from '@/src/api/matchTypes';
import { zoneElementId } from '@/src/lib/drops';

/**
 * Where each seat sits around a table, as seen from one chair.
 *
 * The viewer is at the bottom. Play goes clockwise, and seen from above the
 * player after you in clockwise order sits on your left — so the seats run
 * bottom, left, (top,) right. At three that is the Mariáš table, at four the
 * Bridge one. A spectator, who has no chair, sees the table from the first
 * seat's.
 *
 * Nothing here knows a game. It is only geometry: a module that sends a zone
 * arranged `bySeat` gets its cards placed by it, whatever the cards are for.
 */
export type Compass = 'bottom' | 'left' | 'topLeft' | 'top' | 'topRight' | 'right';

/** Positions by table size, in clockwise order starting from the viewer. */
const AROUND: Record<number, Compass[]> = {
  1: ['bottom'],
  2: ['bottom', 'top'],
  3: ['bottom', 'left', 'right'],
  4: ['bottom', 'left', 'top', 'right'],
  5: ['bottom', 'left', 'topLeft', 'topRight', 'right'],
  6: ['bottom', 'left', 'topLeft', 'top', 'topRight', 'right'],
};

/** The largest table a compass is drawn for; past it, cards sit in a row. */
export const MAX_COMPASS_SEATS = 6;

/**
 * Each seat's position, from `viewerId`'s chair. Seats are in turn order, as
 * the view sends them. Empty when the table is too large to draw as a
 * compass — the caller then lays the cards out in a plain row.
 */
export function seatCompass(seatIds: string[], viewerId: string): Map<string, Compass> {
  const out = new Map<string, Compass>();
  const around = AROUND[seatIds.length];
  if (!around) return out;
  const me = Math.max(0, seatIds.indexOf(viewerId));
  seatIds.forEach((id, i) => {
    out.set(id, around[(i - me + seatIds.length) % seatIds.length]!);
  });
  return out;
}

export type PlacedCard = { card: CardView; at: Compass };

/**
 * A seat-arranged zone's cards, split into those with a place at the table
 * and those without one — a card whose `by` names nobody seated, or no one.
 * The second list is never dropped: the shell draws it in the middle, since
 * a card the server sent is a card the player should see.
 *
 * Order is preserved, which is play order: a later card is drawn over an
 * earlier one where the two overlap, as it lands on a real table.
 */
export function placeBySeat(
  zone: Zone,
  compass: Map<string, Compass>,
): { placed: PlacedCard[]; loose: CardView[] } {
  const placed: PlacedCard[] = [];
  const loose: CardView[] = [];
  for (const card of zone.cards ?? []) {
    const at = card.by ? compass.get(card.by) : undefined;
    if (at) placed.push({ card, at });
    else loose.push(card);
  }
  return { placed, loose };
}

/** Whether the shell should draw this zone as a table rather than by its kind. */
export function isSeatArranged(zone: Zone): boolean {
  return zone.arrange === 'bySeat';
}

/**
 * The element a card played by `playerId` lands on — that seat's own spot in
 * the trick, so a flight ends where the card is then drawn rather than in the
 * middle of the zone.
 */
export const seatSlotElementId = (zoneId: string, playerId: string) =>
  `${zoneElementId(zoneId)}-by-${playerId}`;
