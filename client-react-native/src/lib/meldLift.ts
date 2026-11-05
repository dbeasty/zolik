import type { ActionOffer } from '@/src/api/matchTypes';

/**
 * Whether an offer takes its cards out of a meld card by card — CanastaX's
 * rearrange and poach — rather than out of a hand. Read off the offer's own
 * source zone, so no game is named here.
 *
 * Not a lift: a move whose cards the module already names (`submit`, which is
 * solitaire's), and never an undo. Canasta's "undo meld" and "undo lay-off"
 * name the meld their cards come back off as a `from_meld` source — where they
 * point, not a pick the player makes. Read as lifts, they dropped out of the
 * controls altogether and a meld laid by mistake had no way back.
 */
export function isMeldLift(o: ActionOffer): boolean {
  return o.source?.zone === 'from_meld' && !o.undo && !o.source?.submit?.length;
}
