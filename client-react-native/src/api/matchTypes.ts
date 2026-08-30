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

/**
 * One card as the board shows it.
 *
 * `badgeKeys` mark this particular card — the same idea as `Group.badgeKeys`,
 * one level down, and set only by the module, which is the only side that
 * knows what is worth marking. Žolíky uses it for the card a discard-pile
 * pickup obliges you to lay down: the rule is enforced at the discard, which
 * is the last possible moment to hear about it, so the mark is what turns a
 * refusal into an instruction given while there is still a turn left to act
 * on it — long-press the card to read it; tap to select it for your lay-down.
 *
 * `faceDown` shows this card back-up — a ceremonial turn, not a secret.
 * Hiding is still done the one way it always was, by not sending the card at
 * all; a module sets this only where the value is already public or spent.
 */
export type CardView = { card: string; badgeKeys?: string[]; faceDown?: boolean };

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
 *
 * `requires` names the other cards that have to go in the same action for
 * this one to be legal — the 5 that only extends a run of 7-8-9-10 with the 6
 * alongside it. Empty (the usual case) means the card may go on its own. The
 * set is transitive and closed, so checking it is a membership test, never a
 * question about which ranks sit next to which. `source.cards` deliberately
 * omits these cards: see {@link Selector}.
 */
export type Placement = { card: string; positions?: string[]; requires?: string[] };

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
  /**
   * The cards that may be sent on their own — the placements with no
   * `requires`. Deliberately narrower than `placements`, which also lists
   * cards legal only in company, because this is what gets sent without
   * anyone choosing: {@link isOneTap} turns a one-entry list into a
   * pressable button, and the terminal client submits `cards[:minCards]`
   * sight unseen.
   */
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
   * The written rules that justify `whyNot` at this table — ids of items in
   * the rule index the server serves for this match, never sentences.
   *
   * A code names a *class* of refusal, which is not enough to explain one:
   * INVALID_MELD means different things in two games and DISCARD_LOCKED
   * means something different at every option value. Wording an explanation
   * per code here would be a second, unversioned copy of the rules living on
   * this side. Pointing at a rule the player can already read is not.
   */
  ruleIds?: string[];
  /** What to do instead. A key and params, like every other fact. */
  remedy?: Fact;
  /**
   * Another offer in this same list that performs the remedy, so the reason
   * can carry a working control rather than an instruction to go and find
   * one. Empty when no single offer does it.
   */
  remedyOfferId?: string;
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
  /**
   * The number to print, when that is not `score`.
   *
   * `score` is oriented higher-is-better so the server can rank and record it
   * without a sense of direction, which carries a rummy penalty of 247 as
   * -247. Printing that shows the player the arithmetic. Use
   * `shownScore(standing)` rather than reading either field directly.
   */
  shown?: number;
  won?: boolean;
  labelKey?: string;
  facts?: Fact[];
};

/**
 * One seat's row in one round.
 *
 * `delta` and `total` are oriented higher-is-better, like `Standing.score`, so
 * one component renders a rummy penalty and a chip stack and puts the arrow the
 * right way round for both. `shown` / `shownTotal` print in their place where
 * the game's own number reads the other way — use `shownScore` rather than
 * reading either field directly.
 */
export type RoundScore = {
  playerId: string;
  delta: number;
  total: number;
  shown?: number;
  shownTotal?: number;
  facts?: Fact[];
};

/** One completed round of a match. */
export type RoundResult = {
  /** 1-based, and the same number the header showed while it was being played. */
  number: number;
  /** Who took the round. Possibly nobody — a deal can end exhausted or level. */
  winners?: string[];
  scores: RoundScore[];
  /** True of the round rather than of any one seat. */
  facts?: Fact[];
};

/**
 * A match's round-by-round history, and whether the table is between rounds.
 *
 * Absent for a game with no rounds, which is not the same as a game with none
 * played yet: a game without them sends nothing, a fresh match sends an empty
 * list.
 */
export type RoundLog = {
  rounds: RoundResult[];
  /** The table is sitting between rounds. The server's answer, never inferred. */
  paused?: boolean;
  /** What a round is called in this game — a deal, a hand, a leg. */
  labelKey: string;
  /** Seats that have not yet said to go on. */
  waitingFor?: string[];
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
  rounds?: RoundLog;
};

/** What the client sends back. Built from an offer, never composed by hand. */
export type MatchAction = {
  offerId?: string;
  verb: string;
  cards?: string[];
  target?: string;
  params?: Record<string, string>;
};

/** One titled group of a game's written rules ("Setup", "How a match ends"). */
/**
 * One written rule, addressable.
 *
 * `id` is the sentence's own label key. Written out as a field of its own so
 * that everything pointing at a rule — a refusal's `ruleIds`, a deep link, an
 * anchor on the rules screen — can say "this id" without knowing that ids
 * happen to be label keys.
 */
export type RuleItem = Fact & { id: string };

export type RuleSection = {
  id: string;
  titleKey: string;
  items: RuleItem[];
};

/**
 * A module's written rules, as `/modules/{id}/rules` reports them — resolved
 * against the variation and options a lobby actually chose, so the sentences
 * describe the table being looked at rather than a module's defaults.
 */
export type ModuleRules = {
  moduleId: string;
  variation?: string;
  options?: Record<string, number>;
  sections: RuleSection[];
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
 * choice this helper has no business making. That now includes a card list
 * that does not fit: a selection bigger than the offer will take used to be
 * silently trimmed down to it, which sent whichever card the trim happened to
 * keep rather than the one a person actually picked, and left the rest of the
 * selection on the table looking chosen when it had already been spent. A
 * mismatched selection is refused instead, the same as a drag already refuses
 * to land one — see `fits` in `src/lib/drops.ts`, which this mirrors for the
 * one case a drag never has to face: nobody having chosen anything at all. A
 * missing choice only stands in for the offer's own list when that list is
 * itself the whole submission — no card left over that only a person could
 * have picked between.
 */
export function submissionFor(
  offer: ActionOffer,
  chosen?: { cards?: string[]; params?: Record<string, string> },
): MatchAction | null {
  if (!offer.enabled) return null;

  const action: MatchAction = { offerId: offer.id, verb: offer.verb };

  const need = offer.source?.minCards ?? 0;
  if (need > 0) {
    if (chosen?.cards) {
      const max = offer.source?.maxCards ?? need;
      if (chosen.cards.length < need || chosen.cards.length > max) return null;
      action.cards = chosen.cards;
    } else {
      const listed = offer.source?.cards ?? [];
      if (listed.length !== need) return null;
      action.cards = listed;
    }
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
 * Whether an offer can be sent by pressing it once, with nothing chosen.
 *
 * True when the server enumerated the *whole* submission — Canasta ships the
 * exact cards of a meld, so it is a button. Exactly that many, not merely
 * enough of them: a lay-off with two eligible cards also enumerates a list
 * that is at least as long as `minCards`, but which of the two goes is a
 * choice only a person can make, and a bare `>=` here used to wave that
 * through as one-tap and let the first candidate in the list go instead.
 * False for a rummy meld shape, or anything with a choice to make first.
 */
export function isOneTap(offer: ActionOffer): boolean {
  if (offer.composite) return false;
  if ((offer.params ?? []).length > 0) return false;
  const need = offer.source?.minCards ?? 0;
  if (need === 0) return true;
  return (offer.source?.cards ?? []).length === need;
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
