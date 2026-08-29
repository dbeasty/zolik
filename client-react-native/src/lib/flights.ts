import type { Seat, Zone } from '@/src/api/matchTypes';
import { zoneElementId } from '@/src/lib/drops';

/**
 * Cards seen travelling between zones.
 *
 * The board is server-driven: nothing ever says "a card moved", the next
 * state simply has it somewhere it wasn't. `SettleIn` puts a little motion
 * back at the destination; this module reconstructs the *journey* — by
 * comparing the last board with this one, in the same kind-and-count
 * vocabulary the shell already speaks. A pile that gained its top card
 * gained it from whoever just acted; a stack one card shorter beside a hand
 * one card longer is a card that went from the first to the second. No
 * game is named, and a module added tomorrow gets its cards flown for free.
 *
 * Deliberately conservative: a transition this can't read as one or two
 * cards changing hands (a fresh deal, a reshuffle, a reconnect) plans no
 * flights at all — the board still snaps to the truth, which is the part
 * that was never this module's job.
 */

export type Flight = {
  /** Stable per transition, so replanning the same one cannot double it. */
  id: string;
  /** Element id (drop-registry vocabulary) the card leaves from. */
  fromId: string;
  /** Element id it lands on. */
  toId: string;
  /** The face shown in transit; unset flies back-up. */
  card?: string;
  /** Lands back-up — the closing gesture some tables end on. */
  faceDown?: boolean;
};

export type FlightPlan = {
  flights: Flight[];
  /**
   * How long the newest card's entrance at each destination should wait,
   * keyed by *element id* — so the landing and the entrance read as one
   * motion instead of the card greeting its own arrival.
   */
  holds: ReadonlyMap<string, number>;
};

/** How long a card is in the air. */
export const FLIGHT_MS = 420;
/** The destination's own entrance waits slightly less, so the two overlap. */
export const FLIGHT_HOLD_MS = 360;
/**
 * A flight that has not taken off this long after being planned never does.
 * In a background tab the browser throttles animation frames, so planned
 * flights queue up untaken — and without this, coming back to the tab
 * released the whole backlog at once, a flock of stale cards narrating
 * moves from minutes ago. The board itself is always current; only the
 * narration is skipped.
 */
export const FLIGHT_STALE_MS = 1000;

/** Where a player *is* on screen when their cards aren't: their seat tile. */
export const seatElementId = (playerId: string) => `seat-${playerId}`;

export type BoardLike = {
  zones: Zone[];
  seats?: Seat[];
};

export const EMPTY_FLIGHT_PLAN: FlightPlan = { flights: [], holds: new Map() };

/** At most this many journeys per transition — busier than that isn't one move. */
const MAX_FLIGHTS = 3;

function handZoneOf(board: BoardLike, playerId: string): Zone | undefined {
  return board.zones.find((z) => z.kind === 'hand' && z.ownerId === playerId);
}

export function planFlights(
  prev: BoardLike | null,
  next: BoardLike | null,
  viewerId: string,
): FlightPlan {
  if (!prev || !next || !prev.zones.length || !next.zones.length) return EMPTY_FLIGHT_PLAN;

  const before = new Map(prev.zones.map((z) => [z.id, z]));

  // Per-owner hand-count changes are the plan's connective tissue: they say
  // whose card a pile gained, and whose hand a stack's missing card went to.
  // Every hand changing at once — or any hand by a lot — is a fresh deal,
  // which is the board being replaced rather than a card travelling.
  const handDelta = new Map<string, number>();
  for (const z of next.zones) {
    if (z.kind !== 'hand' || !z.ownerId) continue;
    const was = before.get(z.id);
    if (!was || Math.abs(z.count - was.count) > 4) return EMPTY_FLIGHT_PLAN;
    if (z.count !== was.count) handDelta.set(z.ownerId, z.count - was.count);
  }

  const grew = [...handDelta].filter(([, d]) => d > 0);
  const shrank = [...handDelta].filter(([, d]) => d < 0);

  // The viewer's cards live in their fan; everyone else's live at their seat.
  const placeFor = (playerId: string): string =>
    playerId === viewerId && handZoneOf(next, playerId)
      ? zoneElementId(handZoneOf(next, playerId)!.id)
      : seatElementId(playerId);

  // Who a pile's new top card came from: the one hand that shrank, or —
  // when the actor's hand isn't counted separately (their cards left via a
  // group) — whoever was on turn on the previous board.
  const actorId =
    shrank.length === 1 ? shrank[0]![0] : prev.seats?.find((s) => s.active)?.playerId ?? null;

  const flights: Flight[] = [];
  const holds = new Map<string, number>();
  const add = (f: Omit<Flight, 'id'>, holdId?: string) => {
    // Deterministic per transition: replanning the same one yields the same
    // ids, which is what lets the consumer de-duplicate instead of doubling.
    flights.push({ ...f, id: `${f.fromId}>${f.toId}#${flights.length}` });
    if (holdId) holds.set(holdId, FLIGHT_HOLD_MS);
  };

  for (const z of next.zones) {
    const was = before.get(z.id);
    if (!was) continue;
    const d = z.count - was.count;

    if (z.kind === 'pile') {
      const top = z.cards?.length ? z.cards[z.cards.length - 1] : undefined;
      if (d === 1 && top && actorId) {
        add(
          {
            fromId: placeFor(actorId),
            toId: zoneElementId(z.id),
            card: top.faceDown ? undefined : top.card,
            faceDown: top.faceDown,
          },
          zoneElementId(z.id),
        );
      } else if (d <= -1 && grew.length === 1 && grew[0]![1] === -d) {
        const owner = grew[0]![0];
        const wasTop = was.cards?.length ? was.cards[was.cards.length - 1] : undefined;
        const ownHand = owner === viewerId ? handZoneOf(next, owner) : undefined;
        add(
          {
            fromId: zoneElementId(z.id),
            toId: placeFor(owner),
            card: d === -1 && !wasTop?.faceDown ? wasTop?.card : undefined,
          },
          ownHand ? zoneElementId(ownHand.id) : undefined,
        );
      }
    }

    if (z.kind === 'stack' && d === -1 && grew.length === 1 && grew[0]![1] === 1) {
      const owner = grew[0]![0];
      const ownHand = owner === viewerId ? handZoneOf(next, owner) : undefined;
      add(
        { fromId: zoneElementId(z.id), toId: placeFor(owner) },
        ownHand ? zoneElementId(ownHand.id) : undefined,
      );
    }

    if (z.kind === 'spread' && d >= 1 && shrank.length === 1 && shrank[0]![1] === -d) {
      // The face flown is the newest card of whichever group changed.
      const prevGroups = new Map((was.groups ?? []).map((g) => [g.id, g]));
      let face: string | undefined;
      for (const g of z.groups ?? []) {
        const old = prevGroups.get(g.id);
        if (!old || g.cards.length > old.cards.length) face = g.cards[g.cards.length - 1];
      }
      add({ fromId: placeFor(shrank[0]![0]), toId: zoneElementId(z.id), card: face }, zoneElementId(z.id));
    }
  }

  if (flights.length === 0 || flights.length > MAX_FLIGHTS) return EMPTY_FLIGHT_PLAN;
  return { flights, holds };
}
