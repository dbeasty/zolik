import type { ActionOffer, Fact } from '@/src/api/matchTypes';
import { offerGroupKey } from '@/src/api/matchTypes';

/**
 * The one line above the controls that says what this player can do right
 * now — read off the offers, like everything else in the shell.
 *
 * Two shapes:
 *
 * - An obligation. The player has started something and must finish it or take
 *   it back before the turn can end. The module says so as the remedy on a
 *   refused offer, and names the undo as the way out. That pairing, a remedy
 *   whose offer is an undo, is how the obligation is recognised without
 *   knowing any game's rules.
 * - Otherwise, the moves on offer, by the names their buttons carry. Undos are
 *   left out: taking a move back is always there and never the next step.
 *
 * Null when nothing is on offer, which is every moment that is not this
 * player's turn.
 */
export type TurnStep = { obligation?: Fact; moves: ActionOffer[] };

export function turnStep(offers: ActionOffer[]): TurnStep | null {
  const live = offers.filter((o) => o.enabled);
  if (!live.length) return null;

  const undos = new Set(offers.filter((o) => o.undo).map((o) => o.id));
  const obligation = offers.find(
    (o) => !o.enabled && o.remedy && o.remedyOfferId && undos.has(o.remedyOfferId),
  )?.remedy;

  const seen = new Set<string>();
  const moves: ActionOffer[] = [];
  for (const o of live) {
    if (o.undo) continue;
    const key = offerGroupKey(o);
    if (seen.has(key)) continue;
    seen.add(key);
    moves.push(o);
  }
  return { obligation, moves };
}
