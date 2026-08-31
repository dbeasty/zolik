import type { Zone } from '@/src/api/matchTypes';
import {
  FLIGHT_HOLD_MS,
  FLIGHT_MS,
  FLIGHT_STALE_MS,
  OWN_MOVE_QUIET_MS,
  planFlights,
  seatElementId,
  type BoardLike,
} from '@/src/lib/flights';

/**
 * The journey-reconstruction rules, pinned in the same kind-and-count
 * vocabulary the planner reads. No game is named anywhere in here — a hand,
 * a stack, a pile and a spread describe every table this client shows.
 */

const hand = (id: string, ownerId: string, count: number, cards?: string[]): Zone => ({
  id,
  kind: 'hand',
  ownerId,
  count,
  cards: cards?.map((card) => ({ card })),
});

const stack = (id: string, count: number): Zone => ({ id, kind: 'stack', count });

const pile = (id: string, cards: { card: string; faceDown?: boolean }[]): Zone => ({
  id,
  kind: 'pile',
  count: cards.length,
  cards,
});

const board = (zones: Zone[], activeSeat?: string): BoardLike => ({
  zones,
  seats: activeSeat ? [{ playerId: activeSeat, active: true }, { playerId: 'other' }] : [],
});

const ME = 'me';

describe('planFlights', () => {
  it('plans nothing without a previous board', () => {
    const next = board([hand('hand:me', ME, 5)]);
    expect(planFlights(null, next, ME).flights).toEqual([]);
  });

  it('flies a card from the stack to the viewer hand when they took one', () => {
    const prev = board([stack('supply', 40), hand('hand:me', ME, 5), hand('hand:b', 'b', 5)]);
    const next = board([stack('supply', 39), hand('hand:me', ME, 6), hand('hand:b', 'b', 5)]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights).toHaveLength(1);
    expect(plan.flights[0]).toMatchObject({ fromId: 'zone-supply', toId: 'zone-hand:me' });
    // The card is unseen in transit — the fan reveals it on landing.
    expect(plan.flights[0]!.card).toBeUndefined();
    expect(plan.holds.get('zone-hand:me')).toBeGreaterThan(0);
  });

  it('flies to an opponent seat when their unseen hand took the card', () => {
    const prev = board([stack('supply', 40), hand('hand:me', ME, 5), hand('hand:b', 'b', 5)]);
    const next = board([stack('supply', 39), hand('hand:me', ME, 5), hand('hand:b', 'b', 6)]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights).toHaveLength(1);
    expect(plan.flights[0]).toMatchObject({ fromId: 'zone-supply', toId: seatElementId('b') });
    // Nothing lands anywhere on screen, so nothing is held back.
    expect(plan.holds.size).toBe(0);
  });

  it('flies the shed card face-up from the hand that shrank to the pile', () => {
    const prev = board([pile('laid', [{ card: '2D' }]), hand('hand:me', ME, 6), hand('hand:b', 'b', 5)]);
    const next = board([pile('laid', [{ card: '2D' }, { card: '9S' }]), hand('hand:me', ME, 5), hand('hand:b', 'b', 5)]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights).toHaveLength(1);
    expect(plan.flights[0]).toMatchObject({ fromId: 'zone-hand:me', toId: 'zone-laid', card: '9S' });
    expect(plan.holds.get('zone-laid')).toBeGreaterThan(0);
  });

  it('flies an opponent shed card from their seat', () => {
    const prev = board([pile('laid', [{ card: '2D' }]), hand('hand:me', ME, 5), hand('hand:b', 'b', 6)]);
    const next = board([pile('laid', [{ card: '2D' }, { card: 'KH' }]), hand('hand:me', ME, 5), hand('hand:b', 'b', 5)]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights[0]).toMatchObject({ fromId: seatElementId('b'), toId: 'zone-laid', card: 'KH' });
  });

  it('falls back to the active seat when no hand count changed', () => {
    // A game whose hands are not counted per player still says who is on turn.
    const prev = board([pile('laid', [{ card: '2D' }])], 'b');
    const next = board([pile('laid', [{ card: '2D' }, { card: 'KH' }])], ME);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights[0]).toMatchObject({ fromId: seatElementId('b'), toId: 'zone-laid' });
  });

  it('flies a back-up card when the landing card lies face down', () => {
    const prev = board([pile('laid', [{ card: '2D' }]), hand('hand:me', ME, 5), hand('hand:b', 'b', 1)]);
    const next = board([
      pile('laid', [{ card: '2D' }, { card: '9S', faceDown: true }]),
      hand('hand:me', ME, 5),
      hand('hand:b', 'b', 0),
    ]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights[0]).toMatchObject({ toId: 'zone-laid', faceDown: true });
    expect(plan.flights[0]!.card).toBeUndefined();
  });

  it('flies a picked-up card from the pile to the hand that grew', () => {
    const prev = board([pile('laid', [{ card: '2D' }, { card: '9S' }]), hand('hand:me', ME, 5, ['AH'])]);
    const next = board([pile('laid', [{ card: '2D' }]), hand('hand:me', ME, 6, ['AH', '9S'])]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights[0]).toMatchObject({ fromId: 'zone-laid', toId: 'zone-hand:me', card: '9S' });
  });

  it('flies the newest card of a grown group to its spread', () => {
    const prev = board([
      { id: 'table:b', kind: 'spread', ownerId: 'b', count: 3, groups: [{ id: 'g1', cards: ['5H', '6H', '7H'] }] },
      hand('hand:b', 'b', 6),
      hand('hand:me', ME, 5),
    ]);
    const next = board([
      { id: 'table:b', kind: 'spread', ownerId: 'b', count: 4, groups: [{ id: 'g1', cards: ['5H', '6H', '7H', '8H'] }] },
      hand('hand:b', 'b', 5),
      hand('hand:me', ME, 5),
    ]);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights[0]).toMatchObject({ fromId: seatElementId('b'), toId: 'zone-table:b', card: '8H' });
  });

  it('plans nothing across a fresh deal', () => {
    const prev = board([stack('supply', 3), hand('hand:me', ME, 0), hand('hand:b', 'b', 0)]);
    const next = board([stack('supply', 78), hand('hand:me', ME, 13), hand('hand:b', 'b', 13)]);
    expect(planFlights(prev, next, ME).flights).toEqual([]);
  });

  it('plans nothing when a pile empties back into a stack', () => {
    // No hand changed, so the cards did not go to anyone.
    const prev = board([stack('supply', 0), pile('laid', [{ card: '2D' }, { card: '9S' }]), hand('hand:me', ME, 5)]);
    const next = board([stack('supply', 1), pile('laid', [{ card: '9S' }]), hand('hand:me', ME, 5)]);
    expect(planFlights(prev, next, ME).flights).toEqual([]);
  });

  it('keeps ids stable when the same transition is planned twice', () => {
    const prev = board([stack('supply', 40), hand('hand:me', ME, 5)]);
    const next = board([stack('supply', 39), hand('hand:me', ME, 6)]);
    const a = planFlights(prev, next, ME);
    const b = planFlights(prev, next, ME);
    expect(a.flights.map((f) => f.id)).toEqual(b.flights.map((f) => f.id));
  });
});

describe('a deal', () => {
  const seats = (...ids: string[]) => ids.map((playerId) => ({ playerId }));

  const dealt = (each: number, stackBefore: number, stackAfter: number) => ({
    prev: {
      zones: [stack('s1', stackBefore), hand('h1', ME, 0), hand('h2', 'b', 0), hand('h3', 'c', 0)],
      seats: seats(ME, 'b', 'c'),
    },
    next: {
      zones: [stack('s1', stackAfter), hand('h1', ME, each), hand('h2', 'b', each), hand('h3', 'c', each)],
      seats: seats(ME, 'b', 'c'),
    },
  });

  it('sends one card to every seat, from the stack it came off', () => {
    const { prev, next } = dealt(5, 52, 37);
    const plan = planFlights(prev, next, ME);
    expect(plan.flights).toHaveLength(3);
    expect(new Set(plan.flights.map((f) => f.fromId)).size).toBe(1);
    // The viewer's own cards land in their fan; everyone else's at their seat.
    expect(plan.flights.map((f) => f.toId)).toEqual([
      'zone-h1',
      seatElementId('b'),
      seatElementId('c'),
    ]);
  });

  it('goes round the table in seat order, not zone order', () => {
    const { prev, next } = dealt(5, 52, 37);
    const shuffled = { ...next, zones: [next.zones[0]!, next.zones[2]!, next.zones[3]!, next.zones[1]!] };
    const plan = planFlights(prev, shuffled, ME);
    expect(plan.flights.map((f) => f.toId)).toEqual([
      'zone-h1',
      seatElementId('b'),
      seatElementId('c'),
    ]);
  });

  it("holds the viewer's own entrance until their card lands", () => {
    const { prev, next } = dealt(5, 52, 37);
    expect(planFlights(prev, next, ME).holds.get('zone-h1')).toBeGreaterThan(0);
  });

  it('declines when the count does not add up', () => {
    // Four short of the total: something else took cards too, and a deal this
    // module cannot fully account for is one it does not narrate.
    const { prev, next } = dealt(5, 52, 41);
    expect(planFlights(prev, next, ME).flights).toHaveLength(0);
  });

  it('declines when a hand was not empty to begin with', () => {
    const prev = {
      zones: [stack('s1', 52), hand('h1', ME, 2), hand('h2', 'b', 0)],
      seats: seats(ME, 'b'),
    };
    const next = {
      zones: [stack('s1', 42), hand('h1', ME, 5), hand('h2', 'b', 5)],
      seats: seats(ME, 'b'),
    };
    expect(planFlights(prev, next, ME).flights).toHaveLength(0);
  });

  it('declines when the hands did not all get the same', () => {
    const prev = {
      zones: [stack('s1', 52), hand('h1', ME, 0), hand('h2', 'b', 0)],
      seats: seats(ME, 'b'),
    };
    const next = {
      zones: [stack('s1', 41), hand('h1', ME, 5), hand('h2', 'b', 6)],
      seats: seats(ME, 'b'),
    };
    expect(planFlights(prev, next, ME).flights).toHaveLength(0);
  });

  it('plans the same deal identically twice', () => {
    const { prev, next } = dealt(5, 52, 37);
    expect(planFlights(prev, next, ME).flights.map((f) => f.id)).toEqual(
      planFlights(prev, next, ME).flights.map((f) => f.id),
    );
  });
});

/**
 * The relationships between the timings, which are the part a tempo change
 * can quietly break. Each of these held at the pace the board shipped with;
 * they are pinned so that raising `TEMPO` has to keep them holding.
 */
describe('the timings stay in proportion', () => {
  it('leaves a flight room to take off before it is called stale', () => {
    // FLIGHT_STALE_MS is a deadline on *starting*, not on finishing, so it
    // does not scale with the tempo — but a tempo high enough to approach it
    // would start culling flights that were only ever waiting to be measured.
    expect(FLIGHT_STALE_MS).toBeGreaterThan(FLIGHT_MS);
  });

  it('lands a card before its destination greets it', () => {
    // The hold is what makes the landing and the entrance one motion. If it
    // ever exceeded the flight, the card would appear at the far end first.
    expect(FLIGHT_HOLD_MS).toBeLessThan(FLIGHT_MS);
  });

  it('stays quiet about a carried card until its flight would have finished', () => {
    expect(OWN_MOVE_QUIET_MS).toBeGreaterThan(FLIGHT_MS + FLIGHT_HOLD_MS);
  });
});
