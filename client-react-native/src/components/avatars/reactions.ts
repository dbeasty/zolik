import type { MoveLine } from '@/src/api/matchTypes';
import { sayFor, type Sfx } from '@/src/sound/cues';

/**
 * Which face on the table moves, and how, when the table makes a sound.
 *
 * Deliberately driven by exactly what drives the sounds — `sfxBetween` and
 * `sayFor` — so a face never moves without the board having made a noise for
 * it, and a muted table still shows who just did what. Pure, so it is tested
 * without a screen.
 *
 * - `nod`   a card landed: the player who played it.
 * - `grab`  the pile was taken: the player who took it.
 * - `call`  a play the announcer calls out (skip, draw four, last card, gin…).
 * - `cheer` / `slump`  the match is over: winners, and everybody else.
 */
export type ReactionKind = 'nod' | 'grab' | 'call' | 'cheer' | 'slump';

/** One reaction to play. `at` changes every time, so the same kind twice in a row still replays. */
export type Reaction = { kind: ReactionKind; at: number };

/**
 * The reactions one board change earns, by player id.
 *
 * `fresh` is oldest first, as `newMoveLines` returns it. Only the newest actor
 * moves for a card or a take — a burst of several moves makes one sound, so it
 * makes one movement — and only the newest called-out play is a `call`, the
 * same one the announcer speaks.
 */
export function reactionsFor(fresh: readonly MoveLine[], sfx: Sfx | null): Record<string, ReactionKind> {
  const out: Record<string, ReactionKind> = {};
  const newest = fresh[fresh.length - 1];
  if (sfx && newest) out[newest.playerId] = sfx === 'take' || sfx === 'pick' ? 'grab' : 'nod';
  for (let i = fresh.length - 1; i >= 0; i--) {
    if (sayFor(fresh[i]!.fact)) {
      out[fresh[i]!.playerId] = 'call';
      break;
    }
  }
  return out;
}

/** The end of a match: the winners jump, the rest sag. A drawn match (no winners) is all cheer. */
export function reactionsForEnding(playerIds: readonly string[], winners: readonly string[]): Record<string, ReactionKind> {
  const out: Record<string, ReactionKind> = {};
  for (const id of playerIds) out[id] = winners.length === 0 || winners.includes(id) ? 'cheer' : 'slump';
  return out;
}
