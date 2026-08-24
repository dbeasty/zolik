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

export type Selector = {
  zone: string;
  ownerId?: string;
  meldId?: string;
  cards?: string[];
  placements?: { card: string; positions?: string[] }[];
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
