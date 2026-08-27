import type { ActionOffer, Fact, Placement } from '@/src/api/matchTypes';

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
    if (cards.length === 1) {
      const p = placements.find((x) => x.card === cards[0]);
      if (p?.positions?.length) spot.positions = p.positions;
    }
    spots.push(spot);
  }
  return spots;
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
  return offers.some((o) => {
    const need = o.source?.minCards ?? 0;
    return o.enabled && need > 0 && cards.length >= need && fits(o, cards).ok;
  });
}
