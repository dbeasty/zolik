import type { Fact, Zone } from '@/src/api/matchTypes';

/**
 * Which sound a change on the table deserves.
 *
 * Like `flights.ts` and `changes.ts`, this reads the board rather than the
 * game: the server never says "a card was dropped", the next state simply has
 * more cards in a place. So a card landing somewhere is a card sound, a face-up
 * pile losing cards to another zone is somebody taking the pile, and neither
 * needs to know what game is on the table. Pure, so it is tested without a
 * speaker.
 *
 * Lifting a single card off that pile — a rummy draw from the discards — is
 * `pick`, a shorter sound of its own, so it is told apart from both a draw from
 * the stock and the whole pile going.
 *
 * A move can be both (taking the pile, then laying a meld); the pile wins,
 * because it is the rarer and the more telling of the two.
 */
export type Sfx = 'card' | 'take' | 'pick';

type Board = { zones: Zone[] };

/** More cards than any one move shifts: a deal, a reshuffle — no sound of its own. */
const BULK = 8;

const visible = (z: Zone) => z.cards !== undefined || (z.groups?.length ?? 0) > 0;

function size(z: Zone | undefined): number {
  if (!z) return 0;
  if (z.groups?.length) return z.groups.reduce((n, g) => n + g.cards.length, 0);
  return z.cards ? z.cards.length : z.count;
}

export function sfxBetween(prev: Board | null, next: Board): Sfx | null {
  if (!prev) return null;
  const was = new Map(prev.zones.map((z) => [z.id, z]));
  let gained = 0;
  let pileTaken = 0;
  let hiddenPileGained = false;
  let gainedAway = false;
  for (const z of next.zones) {
    const before = was.get(z.id);
    const delta = size(z) - size(before);
    if (delta > 0) {
      gained += delta;
      if (z.kind === 'pile' || z.kind === 'stack') {
        if (!visible(z)) hiddenPileGained = true;
      } else gainedAway = true;
    } else if (delta < 0 && before && z.kind === 'pile' && visible(before) && size(before) > 0) {
      pileTaken = Math.max(pileTaken, -delta);
    }
  }
  if (gained === 0 || gained > BULK) return null;
  // A visible pile that shrank while a hidden one grew is the discards being
  // turned over to make a new stock, not a take.
  if (pileTaken && gainedAway && !hiddenPileGained) return pileTaken === 1 ? 'pick' : 'take';
  return 'card';
}

/**
 * What the announcer says for a move, as a message key and its params.
 *
 * Only plays a player calls out aloud at a real table are here — a draw
 * penalty, a skip, a reversal, "last card" — and the match is by the move's
 * own label key, the one place a module already says what happened. An
 * unrecognised move is simply not spoken.
 */
export type Say = { key: string; params?: Record<string, unknown> };

const RULES: [RegExp, (f: Fact) => Say][] = [
  [/\.playedDrawFour$/, () => ({ key: 'sound.say.takeFour' })],
  [/\.playedDraw(Colour)?$/, (f) => ({ key: 'sound.say.take', params: { n: f.params?.n ?? 2 } })],
  [/\.playedStack(Colour)?$/, (f) => ({ key: 'sound.say.take', params: { n: f.params?.n ?? 2 } })],
  [/\.playedSkip$/, () => ({ key: 'sound.say.skip' })],
  [/\.playedReverse/, () => ({ key: 'sound.say.reverse' })],
  [/^lastcard\.move\.called$/, () => ({ key: 'sound.say.lastCard' })],
  [/^lastcard\.move\.caught$/, () => ({ key: 'sound.say.caught' })],
  [/^ginrummy\.move\.knocked$/, () => ({ key: 'sound.say.knock' })],
  [/^ginrummy\.move\.wentGin$/, () => ({ key: 'sound.say.gin' })],
];

export function sayFor(fact: Fact): Say | null {
  for (const [re, make] of RULES) if (re.test(fact.labelKey)) return make(fact);
  return null;
}
