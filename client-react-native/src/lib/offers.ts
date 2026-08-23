/**
 * Lookups over the server's legal-action list.
 *
 * The invariant that makes this file worth having: **it contains no rule
 * knowledge.** Not one card rank, meld size, phase name, profile name or
 * "am I allowed to" expression. Every question it answers is answered by
 * reading what the server already decided (see server/internal/rules/offers.go
 * and docs/extensibility-plan.md Phase 1).
 *
 * If you ever find yourself adding `if (phase === ...)` or `state.roundReqMet`
 * to this file, the thing you want is a new field on the offer, not a
 * condition here — otherwise the drift this module exists to end starts over.
 */

import type { ActionOffer, GameState, Placement } from '@/src/api/types';

/** Offer IDs the server guarantees are always present. Mirrors the constants in rules/offers.go. */
export const OFFER = {
  drawDeck: 'draw:deck',
  drawDiscard: 'draw:discard',
  layMeld: 'lay_meld',
  discard: 'discard',
  undoDrawDiscard: 'undo:draw_discard',
  undoLayOff: 'undo:lay_off',
  undoLayMeld: 'undo:lay_meld',
  undoTurn: 'undo:turn',
} as const;

export const layOffOfferId = (meldId: string) => `lay_off:${meldId}`;
export const swapJokerOfferId = (meldId: string) => `swap_joker:${meldId}`;

export function findOffer(state: GameState | null, id: string): ActionOffer | undefined {
  return state?.legalActions?.find((o) => o.id === id);
}

/**
 * Whether the server is currently offering this action.
 *
 * Defaults to `false` when the offer list is missing entirely, which only
 * happens against a server build predating Phase 1 — safest failure mode is
 * an inert control rather than one that sends an action the server rejects.
 */
export function can(state: GameState | null, id: string): boolean {
  return findOffer(state, id)?.enabled ?? false;
}

/** The engine's own reason an action is unavailable, or undefined when it is available. */
export function whyNot(state: GameState | null, id: string): string | undefined {
  const o = findOffer(state, id);
  return o && !o.enabled ? o.whyNot : undefined;
}

/** The cards this offer will accept from the player's hand. */
export function eligibleCards(state: GameState | null, id: string): string[] {
  return findOffer(state, id)?.source?.cards ?? [];
}

/** Whether this specific card is one the offer accepts. */
export function offersCard(state: GameState | null, id: string, card: string): boolean {
  const o = findOffer(state, id);
  return !!o?.enabled && (o.source?.cards ?? []).includes(card);
}

/**
 * Which end(s) of a run this card may extend, as the server resolved it.
 *
 * An empty array means "the server accepts this card but wants no position
 * hint" — a set (which has no ends), or a card whose placement is unambiguous
 * either way. Callers should then send no `position` at all rather than
 * guessing one, which is exactly what the server rejects with WRONG_RUN_END.
 */
export function placementsFor(state: GameState | null, meldId: string): Placement[] {
  return findOffer(state, layOffOfferId(meldId))?.source?.placements ?? [];
}

export function positionsForCard(
  state: GameState | null,
  meldId: string,
  card: string,
): string[] {
  return placementsFor(state, meldId).find((p) => p.card === card)?.positions ?? [];
}

/** Whether any meld on the table is currently accepting a lay-off. */
export function canLayOffAnywhere(state: GameState | null): boolean {
  return (state?.legalActions ?? []).some((o) => o.verb === 'lay_off' && o.enabled);
}

/** Whether any meld on the table is currently accepting a joker swap. */
export function canSwapJokerAnywhere(state: GameState | null): boolean {
  return (state?.legalActions ?? []).some((o) => o.verb === 'swap_joker' && o.enabled);
}

/**
 * Whether a card dragged onto a meld would land at all — either as a lay-off
 * or as a joker swap.
 *
 * The two have to be asked together because they are genuinely independent:
 * the run 8-9-[JKR as T]-J-Q-K takes no lay-off from a hand holding the T
 * (both ends are unrelated ranks) while still offering that exact T as a
 * joker swap. Gating the meld drop zones on lay-off alone made a table like
 * that inert — the card was dragged onto the meld and nothing happened at
 * all, no move, no error.
 */
export function canDropOnMeldAnywhere(state: GameState | null): boolean {
  return canLayOffAnywhere(state) || canSwapJokerAnywhere(state);
}

/**
 * Whether *this* meld will accept a lay-off right now.
 *
 * Note this is deliberately stricter than `canLayOffAnywhere`: the server
 * disables a per-meld offer when nothing in hand fits that particular meld,
 * so a drop zone bound to this answer stops highlighting melds the drop
 * would bounce off.
 */
export function canLayOffOnto(state: GameState | null, meldId: string): boolean {
  return can(state, layOffOfferId(meldId));
}

/** Whether this meld is offering a joker swap, and which cards would take the joker's place. */
export function canSwapJokerOn(state: GameState | null, meldId: string): boolean {
  return can(state, swapJokerOfferId(meldId));
}

export function jokerSwapCards(state: GameState | null, meldId: string): string[] {
  return eligibleCards(state, swapJokerOfferId(meldId));
}
