import type { ActionOffer, Fact, Placement, Zone } from '@/src/api/matchTypes';
import { isOneTap } from '@/src/api/matchTypes';

/**
 * Where the cards in your hand may be dropped, worked out from the offers.
 *
 * This is the part of drag-and-drop that used to be a game's worth of
 * knowledge and is now a lookup. The old screen knew that a card dropped on a
 * meld was a lay-off, that a card dropped on the pile was a discard, and which
 * end of a run a card could extend — three rummy rules, restated on the client,
 * where they could disagree with the server.
 *
 * None of that is here. An offer already says which cards it takes
 * (`source.cards`), where it lands (`target.meldId` for a group, or
 * `target.zoneId` for a whole zone), and, for a card that could go at either
 * end of something, which ends are legal (`placements`). So the question
 * "what can I do with this card, and where do I let go of it?" is answered by
 * filtering the offer list — and a game added tomorrow gets drag-and-drop with
 * no change here, as long as it says where its moves land.
 */

export type DropSpot = {
  offerId: string;
  /**
   * The testID of the element that accepts this drop — `group-<meldId>` for a
   * meld, `zone-<zoneId>` for a whole zone. The client renders both under
   * exactly these ids, which is what lets a drop be hit-tested without any
   * per-game wiring.
   */
  elementId: string;
  /**
   * True when letting go here sends the move. False when the offer wants more
   * cards than are being dragged — a rummy meld needs three and you are
   * holding one — in which case the drop adds to the selection instead, and
   * the offer's own button lights up when it has enough.
   */
  ready: boolean;
  /** Legal placement positions for the dragged card, in rendered order. */
  positions?: string[];
  /**
   * Where each of `positions` sits among the target group's cards — see
   * `Placement.slots`. Same length and order, and absent together with it.
   */
  slots?: number[];
  /**
   * Why letting go here would be refused. Undefined when it would be taken.
   *
   * A refused spot is still a spot. Before this field the list held only
   * targets a drop would be accepted by, so a card let go anywhere else
   * simply snapped home in silence — with the reason already computed one
   * line earlier and thrown away, which is the worst moment in the interface
   * to say nothing: a drag is when a player has the strongest hypothesis
   * about what is legal.
   */
  refusal?: {
    /** The engine's code, when the whole offer is refused. */
    code?: string;
    /** The rules behind it, for the sheet to resolve. */
    ruleIds?: string[];
    remedy?: Fact;
    remedyOfferId?: string;
    /** This side's own reason, when the cards are what does not fit. */
    labelKey?: string;
    params?: Record<string, string | number>;
  };
};

export const zoneElementId = (zoneId: string) => `zone-${zoneId}`;
export const groupElementId = (meldId: string) => `group-${meldId}`;

/** Placements live on whichever selector the module put them on. */
function placementsOf(offer: ActionOffer): Placement[] {
  return offer.source?.placements ?? offer.target?.placements ?? [];
}

/** Whether `wanted` can be drawn from `available`, counting duplicates. */
function coveredBy(wanted: string[], available: string[]): boolean {
  const left = new Map<string, number>();
  for (const c of available) left.set(c, (left.get(c) ?? 0) + 1);
  for (const c of wanted) {
    const n = left.get(c) ?? 0;
    if (n === 0) return false;
    left.set(c, n - 1);
  }
  return true;
}

/** Why a set of cards is not ones this offer could ever send. */
export type Fit =
  | { ok: true }
  | { ok: false; labelKey: string; params?: Record<string, string | number> };

/**
 * Whether these particular cards are ones this offer could ever be sent
 * with — enough of a card, not too many, and (when the offer enumerates its
 * own candidates) only cards on that list. Says nothing about whether there
 * are *enough of them yet*: a drag still gathering cards for a rummy meld and
 * a button that needs a person to finish choosing both ask that separately,
 * because a drag is allowed to stage a partial answer where a button is not.
 *
 * An offer that takes no cards at all (a draw, an undo) has no opinion on any
 * selection — it is not this offer's business what else happens to be picked.
 */
export function fits(offer: ActionOffer, cards: string[]): Fit {
  const need = offer.source?.minCards ?? 0;
  if (need === 0) return { ok: true };

  const max = offer.source?.maxCards ?? need;
  if (cards.length > max) {
    return max === 1
      ? { ok: false, labelKey: 'sel.tooMany.1' }
      : { ok: false, labelKey: 'sel.tooMany.n', params: { n: max } };
  }

  const placements = placementsOf(offer);
  const enumerated = placements.length > 0 ? placements.map((p) => p.card) : offer.source?.cards;
  // No list at all means the offer bounds a shape rather than listing
  // combinations — a rummy meld — and any card in hand may go into it.
  if (enumerated && enumerated.length > 0 && !coveredBy(cards, enumerated)) {
    return { ok: false, labelKey: 'sel.notThese' };
  }
  // Some cards are only legal with company, and the offer says which company:
  // the 5 that extends a run of 7-8-9-10 only when the 6 goes with it. Reading
  // the list off is not deriving a rule — this side still has no idea why
  // those two cards belong together, only that the module said they do.
  //
  // A card may have more than one company that works, and then the offer says
  // so too: `alternatives` holds the other sets, and any one of them held in
  // full is enough. That is what lets a player spend a joker on a gap their
  // hand could also fill naturally — before it, `requires` named the natural
  // card and the joker-and-card pair the player dragged was refused, leaving
  // them to lay the two cards one at a time.
  //
  // Membership, not a multiset: a companion names a rank and a suit, and
  // either copy of a duplicate in a two-deck game satisfies it.
  const picked = new Set(cards);
  for (const p of placements) {
    if (!picked.has(p.card) || !p.requires?.length) continue;
    const ways = [p.requires, ...(p.alternatives ?? [])];
    if (!ways.some((way) => way.every((r) => picked.has(r)))) {
      return { ok: false, labelKey: 'sel.needsCompany' };
    }
  }
  return { ok: true };
}

/**
 * Every place the cards currently being dragged may be let go of.
 *
 * An offer qualifies when it is enabled, it says where it lands, it takes
 * cards from a hand at all, and it accepts these particular ones. Offers that
 * enumerate their cards are checked against that list as a multiset, because
 * two decks are in play in two of these games and "7H" may name either of two
 * cards a player is holding.
 */
export function dropSpotsFor(offers: ActionOffer[], cards: string[]): DropSpot[] {
  if (cards.length === 0) return [];

  const spots: DropSpot[] = [];
  for (const offer of offers) {
    const elementId = offer.target?.meldId
      ? groupElementId(offer.target.meldId)
      : offer.target?.zoneId
        ? zoneElementId(offer.target.zoneId)
        : null;
    if (!elementId) continue;

    // An offer that takes no cards is a button, not a drop target: drawing
    // from the deck is not something you do by dragging a card onto it. Not
    // a refusal either — there was never anywhere to let go — so it is left
    // off the list entirely rather than explained.
    const need = offer.source?.minCards ?? 0;
    if (need === 0) continue;

    // Classified, not filtered. A place a card cannot go is a place with a
    // reason attached, and the caller decides whether to light it up or
    // explain it — see `refusalAt`.
    if (!offer.enabled) {
      spots.push({
        offerId: offer.id,
        elementId,
        ready: false,
        refusal: {
          code: offer.whyNot,
          ruleIds: offer.ruleIds,
          remedy: offer.remedy,
          remedyOfferId: offer.remedyOfferId,
        },
      });
      continue;
    }

    const fit = fits(offer, cards);
    if (!fit.ok) {
      spots.push({
        offerId: offer.id,
        elementId,
        ready: false,
        refusal: { labelKey: fit.labelKey, params: fit.params },
      });
      continue;
    }

    const placements = placementsOf(offer);
    const spot: DropSpot = { offerId: offer.id, elementId, ready: cards.length >= need };
    const placed = placementForSelection(placements, cards);
    if (placed?.positions?.length) {
      spot.positions = placed.positions;
      // Only when the module gave one per position. A half-filled list would
      // be worse than none: the board would draw the gap in the wrong place
      // for the positions it did not cover.
      if (placed.slots?.length === placed.positions.length) spot.slots = placed.slots;
    }
    spots.push(spot);
  }
  return spots;
}

/**
 * The placement this exact selection *is* — which is what says which ends of
 * the target it may be let go on, and whereabouts in it each end sits.
 *
 * A placement's `positions` describe *its own* submission: for an ordinary
 * card that is the card by itself, and for one with `requires` it is that card
 * together with the cards it names — or with any of its `alternatives`, which
 * reach across the same gap and so grow the same end (the server sweeps that,
 * in TestLayOffPlacements_EveryCompanionGrowsTheSameEnd). So the hint for a
 * selection is the hint of the placement whose submission the selection *is* —
 * nothing else composes.
 *
 * Two cards that each separately could extend either end do not between them
 * make a submission that extends either end, which is why intersecting the
 * per-card hints is wrong and why anything unrecognised sends no hint at all.
 * Saying nothing is what the module already does when a submission grows a run
 * at both ends, and the server treats an absent position as "no constraint".
 */
function placementForSelection(placements: Placement[], cards: string[]): Placement | undefined {
  const picked = new Set(cards);
  if (picked.size !== cards.length) return undefined; // a duplicate names no one card
  for (const p of placements) {
    if (!picked.has(p.card) || !p.positions?.length) continue;
    for (const way of [p.requires ?? [], ...(p.alternatives ?? [])]) {
      const submission = new Set([p.card, ...way]);
      if (submission.size !== picked.size) continue;
      if ([...picked].every((c) => submission.has(c))) return p;
    }
  }
  return undefined;
}

/**
 * The spots a drop would actually be taken by — what lights up, and what a
 * release sends.
 *
 * Split out because "where may this go" and "where has this been forbidden"
 * are now the same list, and lighting up a refused target would be worse than
 * the silence this replaced.
 */
export function takeableSpots(spots: DropSpot[]): DropSpot[] {
  return spots.filter((s) => !s.refusal);
}

/** The refusal for one element, if letting go there would be refused. */
export function refusalAt(spots: DropSpot[], elementId: string): DropSpot['refusal'] | undefined {
  return spots.find((s) => s.elementId === elementId && s.refusal)?.refusal;
}

/** The spot for one element, if the dragged cards may be dropped on it. */
export function spotAt(spots: DropSpot[], elementId: string): DropSpot | undefined {
  return spots.find((s) => s.elementId === elementId && !s.refusal);
}

/**
 * Which of several legal positions a drop at `y` means.
 *
 * The list is in rendered order, so this is just "which slice of the target
 * did the pointer land in" — the top of a two-ended run's group is the first
 * entry, the bottom the last. It never has to know those are the low and
 * high ends of a run, which is the whole point.
 *
 * Vertical, not horizontal: a group is drawn as a stack overlapping top to
 * bottom (see `ZoneView`'s `stackedCards`), one card wide regardless of how
 * long the run is, so a slice of its *width* is a sliver with nothing on
 * screen to tell two ends apart. Its height is exactly the thing that grows
 * with the run and reads as "top" and "bottom" to whoever is looking at it.
 */
export function positionAt(
  positions: string[] | undefined,
  y: number,
  rect: { y: number; height: number },
): string | undefined {
  if (!positions?.length) return undefined;
  if (positions.length === 1) return positions[0];
  if (rect.height <= 0) return positions[0];

  const share = (y - rect.y) / rect.height;
  const index = Math.floor(share * positions.length);
  return positions[Math.max(0, Math.min(index, positions.length - 1))];
}

/**
 * Whether these cards are a submission this offer could be sent with *right
 * now* — `fits` plus the count `fits` deliberately leaves out.
 *
 * The two were always meant to be asked together wherever a press is about to
 * happen (see `fits`), and for a long time only the drag path asked the second
 * one. A control that asked `fits` alone stayed lit over a selection still
 * short of `minCards`, and the press then died in silence: `submissionFor`
 * refuses a submission under the offer's own floor, so nothing was sent,
 * nothing was refused, and nothing on screen said why. The way that was found
 * is the worst case of it — an offer whose floor is six cards, where picking
 * any one of them left a button that looked ready and did nothing at all.
 */
export function readyWith(offer: ActionOffer, cards: string[]): Fit {
  const fit = fits(offer, cards);
  if (!fit.ok) return fit;
  const need = offer.source?.minCards ?? 0;
  if (cards.length < need) return { ok: false, labelKey: 'sel.needMore', params: { n: need } };
  return { ok: true };
}

/**
 * Whether some enabled offer that actually takes cards is ready to send
 * these right now — not merely compatible with them eventually, which a
 * fresh meld-in-progress always is (`fits` has no opinion on a selection
 * still short of an offer's minimum), and not an offer with no opinion on
 * any selection at all (a draw, an undo — see `fits`), which every selection
 * trivially "fits" by never objecting to one.
 *
 * Built for the one place selecting a second card has to guess what the
 * first was for: a card that just arrived in hand lands pre-picked, so a
 * second tap either joins it (this says yes — a multi-card lay-off is ready
 * the moment two eligible cards are both picked) or was meant to replace it
 * (this says no — an unfinished meld or a full hand isn't a reason to keep
 * the first pick around).
 */
export function someOfferReady(offers: ActionOffer[], cards: string[]): boolean {
  return offers.some((o) => o.enabled && (o.source?.minCards ?? 0) > 0 && readyWith(o, cards).ok);
}

/**
 * The piles a press acts *from* — the other half of "a card in hand is a card
 * looking for somewhere to go".
 *
 * Everything else in this file answers "where may these cards be let go of",
 * which is a question about an offer's *target*. A draw has no cards to let go
 * of and nothing to choose: its whole move is "take from there", and the only
 * thing on screen that says so is the pile it names as its source. Until this
 * existed the deck was scenery — the one way to draw was a button in the
 * control bar, several rows from the cards it deals into, and tapping the deck
 * itself did nothing at all.
 *
 * Still no rule is derived here. An offer qualifies when the server has said
 * all four of these things about it:
 *
 *   - it is enabled, and one tap sends it (nothing to compose, no form);
 *   - it takes no cards from a hand, so there is nothing to have picked first;
 *   - it names a rendered zone it comes *from*, drawn as a pile or a stack —
 *     the two kinds a person can point at and mean "that one";
 *   - it lands in the viewer's own hand, which is what makes it a draw rather
 *     than some other card-less move that happens to mention a pile (Canasta's
 *     undo of a capture names the discard pile too, and undoing is not what a
 *     player tapping the pile means).
 *
 * Where two such offers name the same pile, neither is offered: a press has
 * exactly one meaning, and guessing which of two moves was meant is the
 * mistake this whole protocol exists to avoid. The control bar still lists
 * both, named, which is the right place for a choice.
 */
export function sourceSpotsFor(offers: ActionOffer[], zones: Zone[], viewerId: string): DropSpot[] {
  const byId = new Map(zones.map((z) => [z.id, z]));

  const claims = new Map<string, DropSpot[]>();
  for (const offer of offers) {
    if (!offer.enabled) continue;
    // Cards to pick first: that is a selection, and the target end of this
    // file already handles it.
    if ((offer.source?.minCards ?? 0) > 0) continue;
    // A combination to compose or a number to fill in — not a press.
    if (!isOneTap(offer)) continue;

    const fromId = offer.source?.zoneId;
    const from = fromId ? byId.get(fromId) : undefined;
    if (!from || (from.kind !== 'pile' && from.kind !== 'stack')) continue;

    const to = offer.target?.zoneId ? byId.get(offer.target.zoneId) : undefined;
    if (!to || to.kind !== 'hand' || to.ownerId !== viewerId) continue;

    const spot: DropSpot = { offerId: offer.id, elementId: zoneElementId(from.id), ready: true };
    claims.set(from.id, [...(claims.get(from.id) ?? []), spot]);
  }

  return [...claims.values()].filter((s) => s.length === 1).map((s) => s[0]!);
}
