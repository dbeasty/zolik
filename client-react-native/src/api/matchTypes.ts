/**
 * The generic match protocol, as the client sees it.
 *
 * These types are a transcription of `server/internal/module/protocol.go` and
 * nothing else. There is deliberately not one game's noun in this file: no
 * meld, no suit, no canasta, no blind, no pot. A board is zones, a table is
 * seats, and what a player may do is offers.
 *
 * Compare `types.ts`'s `GameState`, which has twenty-four rummy-named fields.
 * That shape can describe exactly one game; this one describes any of them,
 * which is what lets a single screen play all four.
 */

/** A labelled value, resolved by the server and rendered by us. */
export type Fact = {
  labelKey: string;
  value?: string;
  params?: Record<string, unknown>;
};

export type CardView = { card: string };

/** Cards within a zone that belong together — a meld, a trick, a board. */
export type Group = {
  id: string;
  kind?: string;
  cards: string[];
  /** Keys for anything worth marking on the group. Keys, never text. */
  badgeKeys?: string[];
};

/**
 * What sort of place a zone is, which is all the shell needs to lay it out.
 *
 * `pile` cards are ordered bottom to top, so the *last* one is the top card —
 * the one a discard pile is really about. A module may send the whole pile or
 * only its top; that is a choice about what is public, not about what is drawn.
 */
export type ZoneKind = 'hand' | 'stack' | 'pile' | 'spread';

/**
 * One area of the board.
 *
 * A hidden zone sends `count` and no `cards` — which is the whole anti-cheat
 * surface, decided by the server because only it knows what is secret in a
 * given game.
 */
export type Zone = {
  id: string;
  kind: ZoneKind;
  ownerId?: string;
  labelKey?: string;
  cards?: CardView[];
  count: number;
  groups?: Group[];
};

/** One player as the board shows them: whose turn, and their own numbers. */
export type Seat = {
  playerId: string;
  active?: boolean;
  labelKeys?: string[];
  facts?: Fact[];
};

export type ViewModel = {
  zones: Zone[];
  seats?: Seat[];
  header?: Fact[];
  status?: Fact[];
  prompts?: Fact[];
};

export type ParamKind = 'choice' | 'int';

/** A non-card input an offer needs: a named choice, or a number in a range. */
export type ParamSpec = {
  name: string;
  kind?: ParamKind;
  labelKey: string;
  choices?: { value: string; labelKey: string }[];
  min?: number;
  max?: number;
  step?: number;
  default?: number;
};

/**
 * One card an offer accepts, and where in the target it may go.
 *
 * `positions` is ordered to match the rendered order of the target group's
 * cards, so a drop on the left half means the first entry and the right half
 * the last — without this side knowing what either name means. A chosen
 * position is submitted under {@link POSITION_PARAM}.
 */
export type Placement = { card: string; positions?: string[] };

/** The parameter a chosen {@link Placement} position travels back under. */
export const POSITION_PARAM = 'position';

export type Selector = {
  /** What sort of place this is: a hand, a deck, a discard pile, a meld. */
  zone: string;
  ownerId?: string;
  meldId?: string;
  /**
   * The *rendered* zone — the same string as a `Zone.id` in the view — where
   * `zone` alone only says what sort of place it is. It is what makes a drop
   * target addressable: which zone a move lands on is a fact about the game,
   * so the server answers it rather than this side guessing from the verb.
   *
   * `meldId` addresses a group inside a zone and is enough on its own.
   */
  zoneId?: string;
  cards?: string[];
  placements?: Placement[];
  minCards?: number;
  maxCards?: number;
};

/**
 * One affordance the interface may present.
 *
 * The server always sends the full set, disabled entries included, each with
 * the engine's own reason — because "greyed out, and here is why" is a UI
 * requirement and an omitted offer is indistinguishable from a client bug.
 */
export type ActionOffer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  /**
   * Names this control when the verb cannot.
   *
   * A control is labelled from its verb, which works until a module offers two
   * of the same verb at once — Žolíky draws from the deck and from the discard
   * pile, both "draw" — and then a player is looking at two buttons that say
   * the same word and do different things. A key, never a sentence.
   */
  labelKey?: string;
  source?: Selector;
  target?: Selector;
  params?: ParamSpec[];
  /** Output shown on the control itself — what this move costs or is worth. */
  facts?: Fact[];
  /**
   * The submission is a *combination* the server does not enumerate, so a
   * person has to compose it from `source.cards`. True for a rummy meld shape;
   * false for everything a button can send in one tap.
   */
  composite?: boolean;
};

/** One row of a scoreboard, in a shape no game owns. */
export type Standing = {
  playerId: string;
  rank: number;
  score: number;
  won?: boolean;
  labelKey?: string;
  facts?: Fact[];
};

export type MatchPlayer = { id: string; name: string; isAI: boolean };

export type MatchState = {
  type: 'match_state';
  matchId: string;
  moduleId: string;
  variation?: string;
  status: 'lobby' | 'active' | 'completed' | 'suspended' | string;
  /** What the lobby chose, echoed back — enough to set the same table again. */
  options?: Record<string, number>;
  joinCode?: string;
  hostId?: string;
  winnerId?: string;
  winners?: string[];
  suspendedPlayer?: string;
  players: MatchPlayer[];
  view: ViewModel;
  legalActions: ActionOffer[];
  standings?: Standing[];
};

/** What the client sends back. Built from an offer, never composed by hand. */
export type MatchAction = {
  offerId?: string;
  verb: string;
  cards?: string[];
  target?: string;
  params?: Record<string, string>;
};

/** A module's self-description, as `/modules` reports it. */
export type MatchModule = {
  id: string;
  label: string;
  minPlayers: number;
  maxPlayers: number;
  variations?: {
    id: string;
    label: string;
    summary?: Fact[];
    defaults?: Record<string, number>;
  }[];
  options?: {
    name: string;
    type: string;
    label: string;
    help?: string;
    choices: { value: number; label: string }[];
  }[];
};

/**
 * Builds the submission an offer describes, from what the offer declares.
 *
 * The client-side twin of the server's `module.SubmissionFor`, and held to the
 * same discipline: cards from the offer's own selector, the target it names, a
 * value for each parameter it declares. It knows what none of them mean.
 *
 * Returns null when the offer needs a person — a composite combination, or a
 * choice this helper has no business making.
 */
export function submissionFor(
  offer: ActionOffer,
  chosen?: { cards?: string[]; params?: Record<string, string> },
): MatchAction | null {
  if (!offer.enabled) return null;

  const action: MatchAction = { offerId: offer.id, verb: offer.verb };

  const need = offer.source?.minCards ?? 0;
  if (need > 0) {
    const cards = chosen?.cards ?? offer.source?.cards ?? [];
    if (cards.length < need) return null;
    const max = offer.source?.maxCards ?? need;
    action.cards = chosen?.cards ? cards.slice(0, Math.max(need, Math.min(cards.length, max))) : cards.slice(0, need);
  }
  if (offer.target?.meldId) action.target = offer.target.meldId;

  for (const p of offer.params ?? []) {
    const supplied = chosen?.params?.[p.name];
    const value = supplied ?? defaultParam(p);
    if (value === undefined) return null;
    action.params = { ...(action.params ?? {}), [p.name]: value };
  }
  return action;
}

/** A legal starting value for a parameter: the server's own default. */
export function defaultParam(p: ParamSpec): string | undefined {
  if (p.kind === 'int') {
    const min = p.min ?? 0;
    const max = p.max ?? min;
    const d = p.default ?? min;
    return String(Math.min(Math.max(d, min), max));
  }
  return p.choices?.[0]?.value;
}

/**
 * Whether an offer can be sent by pressing it once.
 *
 * True when the server enumerated the whole submission — Canasta ships the
 * exact cards of a meld, so it is a button. False for a rummy meld shape, or
 * anything with a choice to make first.
 */
export function isOneTap(offer: ActionOffer): boolean {
  if (offer.composite) return false;
  if ((offer.params ?? []).length > 0) return false;
  const need = offer.source?.minCards ?? 0;
  if (need === 0) return true;
  return (offer.source?.cards ?? []).length >= need;
}

/**
 * The key that ties several offers together as "the same control, aimed at
 * different targets" — same verb, same label, different `target.meldId`. A
 * module that names two same-verb offers distinctly (Žolíky's two draws) is
 * unaffected: they get different keys and are never folded into one.
 */
export function offerGroupKey(offer: ActionOffer): string {
  return offer.labelKey ?? `verb.${offer.verb}`;
}

/**
 * Whether the current selection, on its own, says this is the one member of
 * its group that was meant — without a person having to pick a target by
 * hand. Mirrors what a press would actually send: some of the selection is
 * among this offer's own eligible cards, or nothing is selected and this
 * offer needs no choice at all to go.
 */
export function offerMatchesSelection(offer: ActionOffer, selected: string[]): boolean {
  if (!offer.enabled) return false;
  if (selected.length === 0) return isOneTap(offer);
  return (offer.source?.cards ?? []).some((c) => selected.includes(c));
}

/**
 * Which of `selected` this offer would actually take. An offer that
 * enumerates its cards (Canasta's melds) only takes those; a composite offer
 * with no list bounds a shape rather than a combination — a Žolíky meld, any
 * card in hand may go into it. Mirrors `dropSpotsFor` in `drops.ts`, which
 * matches the same offers against a drag rather than a tap-selection.
 */
export function eligibleCards(offer: ActionOffer, selected: string[]): string[] {
  const allowed = offer.source?.cards;
  if (allowed && allowed.length > 0) return selected.filter((c) => allowed.includes(c));
  return selected;
}
