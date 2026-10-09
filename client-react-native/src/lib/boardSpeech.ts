import { cardSpokenName, cardsSpoken } from '@/src/a11y/cardNames';
import type {
  ActionOffer,
  Fact,
  Group,
  MatchAction,
  MatchPlayer,
  MatchState,
  MoveLine,
  Seat,
  Standing,
  Zone,
} from '@/src/api/matchTypes';
import { dropSpotsFor, takeableSpots, type DropSpot } from '@/src/lib/drops';
import { t } from '@/src/lib/i18n';
import { isDealerLabel, label, playerName, shownScore, spokenFactText } from '@/src/lib/labels';
import { partnerText } from '@/src/lib/sides';
import { turnStep } from '@/src/lib/turnStep';

/**
 * The board, said rather than drawn.
 *
 * Everything a sighted player takes in at a glance — whose turn it is, what is
 * on the pile, which meld a card would fit — is spread across the screen as
 * position, colour and shape. A screen reader gets none of that unless it is
 * written down, and this file is where it is written down: one sentence per
 * thing on the board, built from the same fields the board itself is drawn
 * from, worded by the bundle.
 *
 * Nothing here knows a game. A pile is "a pile with a top card and a count",
 * a meld is "a group of a kind with an owner", a seat is "the facts the seat
 * carries". Where the board would have to know a rule to say something — that
 * a run is "of Hearts", "Five to Nine" — this says the cards instead
 * ("Five of Hearts to Nine of Hearts"), which is true whatever the rule is.
 *
 * Pure functions over the match state, so they can be tested without a
 * screen; the components only decide where each sentence is attached.
 */

/** "1 card", "11 cards". */
export function cardsPhrase(n: number): string {
  return n === 1 ? t('a11y.board.cards.one') : t('a11y.board.cards.n', { n });
}

/** The zone's own name, with its owner's in front when it is somebody else's. */
export function zoneName(zone: Zone, players: MatchPlayer[], viewerId: string): string {
  const own = label(zone.labelKey) || zone.id;
  if (!zone.ownerId || zone.ownerId === viewerId) return own;
  return t('a11y.board.zone.owned', { name: playerName(players, zone.ownerId), zone: own });
}

/**
 * A zone as one sentence: "Discard pile, top card Seven of Clubs, 14 cards".
 *
 * Face-down cards are counted, never named — the value of a card the viewer
 * cannot see is not sent at all, and a back the board shows "ceremonially"
 * (`faceDown`) is still a back.
 */
export function zoneSpokenLabel(zone: Zone, players: MatchPlayer[], viewerId: string): string {
  const name = zoneName(zone, players, viewerId);
  const cards = zone.cards ?? [];
  const faceUp = cards.filter((c) => !c.faceDown);
  const down = Math.max(0, zone.count - faceUp.length);
  if (zone.kind === 'pile') {
    const top = faceUp[faceUp.length - 1];
    if (!top && zone.count <= 0) return t('a11y.board.pile.empty', { pile: name });
    if (!top) return t('a11y.board.pile.count', { pile: name, cards: cardsPhrase(zone.count) });
    return t('a11y.board.pile.top', {
      pile: name,
      card: cardSpokenName(top.card),
      cards: cardsPhrase(zone.count),
    });
  }
  if (zone.kind === 'stack') {
    return zone.count > 0
      ? t('a11y.board.pile.count', { pile: name, cards: cardsPhrase(zone.count) })
      : t('a11y.board.pile.empty', { pile: name });
  }
  const groups = zone.groups ?? [];
  if (groups.length) {
    return t('a11y.board.zone.groups', { zone: name, n: groups.length });
  }
  if (faceUp.length && faceUp.length <= 8) {
    const shown = t('a11y.board.zone.cards', { zone: name, cards: cardsSpoken(faceUp.map((c) => c.card)) });
    return down > 0 ? `${shown}, ${t('a11y.board.pile.faceDown', { n: down })}` : shown;
  }
  return t('a11y.board.pile.count', { pile: name, cards: cardsPhrase(zone.count) });
}

/** What kind of group this is, in words: "Run", "Set", "Column" — "Meld" when the module did not say. */
export function groupKindName(group: Group): string {
  return group.kind ? label(`a11y.board.group.kind.${group.kind}`) : t('a11y.board.group.meld');
}

/** The cards of a group, short enough to hear: all of them up to four, the ends past that. */
export function groupCardsPhrase(cards: readonly string[]): string {
  if (!cards.length) return t('a11y.board.group.empty');
  if (cards.length <= 4) return cardsSpoken(cards);
  return t('a11y.board.group.span', {
    first: cardSpokenName(cards[0]!),
    last: cardSpokenName(cards[cards.length - 1]!),
    cards: cardsPhrase(cards.length),
  });
}

/**
 * A meld as one sentence: "Run: Five of Hearts to Nine of Hearts, 5 cards,
 * Anna's, complete".
 */
export function groupSpokenLabel(group: Group, ownerName?: string): string {
  const parts = [t('a11y.board.group.label', { kind: groupKindName(group), cards: groupCardsPhrase(group.cards) })];
  if (group.hidden) parts.push(t('a11y.board.pile.faceDown', { n: group.hidden }));
  if (ownerName) parts.push(t('a11y.board.group.owner', { name: ownerName }));
  if (group.complete) parts.push(t('a11y.board.group.complete'));
  for (const b of group.badgeKeys ?? []) parts.push(label(b));
  return parts.join(', ');
}

/** The zone holding a group, if any — a meld's owner is its zone's owner. */
export function zoneOfGroup(zones: Zone[], groupId: string): { zone: Zone; group: Group } | undefined {
  for (const zone of zones) {
    const group = (zone.groups ?? []).find((g) => g.id === groupId);
    if (group) return { zone, group };
  }
  return undefined;
}

/** Whose a zone is, as a name to put after a meld — "you" for the viewer's own. */
export function ownerNameOf(zone: Zone, players: MatchPlayer[], viewerId: string): string | undefined {
  if (!zone.ownerId) return undefined;
  return zone.ownerId === viewerId ? t('match.you') : playerName(players, zone.ownerId);
}

/**
 * The name of a place a card can go, from the element id an offer lands on
 * (see `groupElementId` / `zoneElementId`): "Run: Five of Hearts to Seven of
 * Hearts, Anna's", "Discard pile".
 */
export function targetName(elementId: string, zones: Zone[], players: MatchPlayer[], viewerId: string): string {
  if (elementId.startsWith('group-')) {
    const found = zoneOfGroup(zones, elementId.slice('group-'.length));
    if (found) return groupSpokenLabel(found.group, ownerNameOf(found.zone, players, viewerId));
  }
  if (elementId.startsWith('zone-')) {
    const zone = zones.find((z) => z.id === elementId.slice('zone-'.length));
    if (zone) return zoneName(zone, players, viewerId);
  }
  return elementId;
}

/** One way to play a card or a selection: an offer, where it lands, and which end. */
export type PlayOption = {
  /** Stable within one board: the offer and the end. */
  key: string;
  spot: DropSpot;
  position?: string;
  /** Where it goes, in words. */
  target: string;
  /** The whole choice, as a control would be named: "Play to Discard pile". */
  label: string;
};

/**
 * Every place these cards may go right now, and why not when nowhere.
 *
 * The same answer a drag lights up (`dropSpotsFor`), listed: a spot that wants
 * more cards than these is listed too, flagged not `ready`, so a player can be
 * told "pick more" rather than "no". A target with a choice of ends is one
 * option per end, because a keyboard has no "top half of the meld" to aim at.
 */
export function playOptionsFor(
  offers: ActionOffer[],
  cards: string[],
  zones: Zone[],
  players: MatchPlayer[],
  viewerId: string,
): { options: PlayOption[]; partial: PlayOption[]; refusal?: DropSpot['refusal'] } {
  const spots = dropSpotsFor(offers, cards);
  const takeable = takeableSpots(spots);
  const options: PlayOption[] = [];
  const partial: PlayOption[] = [];
  for (const spot of takeable) {
    const target = targetName(spot.elementId, zones, players, viewerId);
    const into = spot.ready ? options : partial;
    if ((spot.positions?.length ?? 0) > 1) {
      for (const position of spot.positions!) {
        into.push({
          key: `${spot.offerId}|${position}`,
          spot,
          position,
          target,
          label: t('a11y.board.playToAt', {
            target,
            position: label(`a11y.board.position.${position}`),
          }),
        });
      }
    } else {
      into.push({
        key: spot.offerId,
        spot,
        position: spot.positions?.[0],
        target,
        label: t('a11y.board.playTo', { target }),
      });
    }
  }
  const refusal = takeable.length ? undefined : spots.find((s) => s.refusal)?.refusal;
  return { options, partial, refusal };
}

/** The flags a card in hand can carry besides its name. */
export type HandCardState = {
  justDrawn?: boolean;
  marked?: boolean;
  playable?: boolean;
};

/**
 * A card in the viewer's hand: "Seven of Clubs, 3 of 11, just drawn, can be
 * played". Selected is not in here: it is the control's own state
 * (`aria-selected`, `accessibilityState.selected`), which a screen reader
 * already says — in the name as well, it would be said twice.
 */
export function handCardLabel(card: string, index: number, count: number, state: HandCardState = {}): string {
  const parts = [cardSpokenName(card), t('a11y.board.card.position', { n: index + 1, count })];
  if (state.justDrawn) parts.push(t('a11y.board.card.justDrawn'));
  if (state.marked) parts.push(t('a11y.board.card.marked'));
  if (state.playable) parts.push(t('a11y.board.card.playable'));
  return parts.join(', ');
}

/** "Your hand, 11 cards." */
export function handSpokenLabel(title: string, count: number): string {
  return t('a11y.board.hand.label', { title, cards: cardsPhrase(count) });
}

/** What a card's tooltip adds to its name: where it can go now. */
export function cardTargetsText(options: PlayOption[], partial: PlayOption[]): string {
  const names = [...new Set([...options, ...partial].map((o) => o.target))];
  return names.length ? t('a11y.board.card.fits', { targets: names.join('; ') }) : '';
}

/**
 * A seat as one sentence: "Anna, bot (Medium), 42 Points, Cards 7, dealer,
 * their turn". Everything in it is something the seat or its player already
 * carries; nothing is counted or inferred here.
 */
export function seatSpokenLabel(opts: {
  seat: Seat;
  seats: Seat[];
  players: MatchPlayer[];
  viewerId: string;
  standing?: Standing;
}): string {
  const { seat, seats, players, viewerId, standing } = opts;
  const player = players.find((p) => p.id === seat.playerId);
  const isMe = seat.playerId === viewerId;
  const name = playerName(players, seat.playerId);
  const parts = [isMe ? `${name} ${t('match.youSuffix')}` : name];
  if (player?.isAI) {
    parts.push(
      player.skill
        ? t('a11y.board.seat.bot', { skill: t(`setup.botSkill.${player.skill}`) })
        : t('a11y.board.seat.botPlain'),
    );
  } else if (player?.isAgent) {
    parts.push(player.agentLabel ? t('a11y.board.seat.agent', { label: player.agentLabel }) : t('a11y.board.seat.agentPlain'));
  }
  if (player?.satOut) parts.push(t('a11y.board.seat.paused'));
  else if (player?.simplified) parts.push(t('seat.simplifiedHint'));
  if (standing) parts.push(scoreText(standing));
  for (const f of seat.facts ?? []) parts.push(spokenFactText(f, players));
  for (const key of seat.labelKeys ?? []) parts.push(label(key));
  const partners = partnerText(seats, players, seat.playerId, viewerId);
  if (partners) parts.push(t('match.teammates', { names: partners }));
  if (seat.active) parts.push(isMe ? t('a11y.board.seat.yourTurn') : t('a11y.board.seat.theirTurn'));
  return parts.join(', ');
}

/** A standing's number and what it counts: "42 Penalty", "980 Points". */
export function scoreText(standing: Standing): string {
  const what = label(standing.labelKey);
  return what
    ? t('a11y.board.seat.score', { score: shownScore(standing), label: what })
    : t('a11y.board.seat.points', { score: shownScore(standing) });
}

/** Whether a seat label key is one of the marks worth a tooltip: the dealer's button. */
export { isDealerLabel };

/**
 * The moves in `next` that were not in `prev`, oldest first.
 *
 * The server sends the last few moves as a window, not as a stream, so "what
 * is new" is what this window holds that the last one did not — counted, not
 * merely compared, because the same sentence can genuinely happen twice in a
 * row ("Anna drew from the stock").
 */
export function newMoveLines(prev: readonly MoveLine[] | undefined, next: readonly MoveLine[] | undefined): MoveLine[] {
  const after = next ?? [];
  if (!prev) return [];
  const before = prev;
  const keyOf = (m: MoveLine) => JSON.stringify([m.playerId, m.fact.labelKey, m.fact.params ?? null, m.fact.value ?? null]);
  // The longest tail of `before` that `after` starts with is what both
  // windows share; whatever follows it in `after` is new.
  const a = before.map(keyOf);
  const b = after.map(keyOf);
  for (let k = Math.min(a.length, b.length); k > 0; k--) {
    let same = true;
    for (let i = 0; i < k; i++) {
      if (a[a.length - k + i] !== b[i]) {
        same = false;
        break;
      }
    }
    if (same) return after.slice(k);
  }
  // Nothing shared: a new round's window, or a burst longer than the window.
  // Everything in it is news; a long burst is summarised by `announce`'s own
  // queue, which keeps the newest.
  return after.slice();
}

/** "Your turn: Draw · Take the discard" — or what the table is holding the player to. */
export function yourTurnText(offers: ActionOffer[], players: MatchPlayer[]): string {
  const step = turnStep(offers);
  if (!step) return '';
  if (step.obligation) return spokenFactText(step.obligation, players);
  return t('step.yourTurn', {
    moves: step.moves.map((o) => label(o.labelKey ?? `verb.${o.verb}`) || o.verb).join(', '),
  });
}

/**
 * The whole table, on request: whose turn, the scores, what is on the piles
 * and the board, the viewer's own melds and how many cards they hold.
 *
 * The sentences are the ones the board already speaks one at a time, put in
 * the order a player asks them in — "is it me? how am I doing? what's on the
 * pile? what have I got?".
 */
export function readTableText(state: MatchState, viewerId: string): string {
  const players = state.players;
  const zones = state.view?.zones ?? [];
  const seats = state.view?.seats ?? [];
  const out: string[] = [];

  const active = seats.find((s) => s.active);
  if (state.status === 'completed') out.push(t('match.over'));
  else if (active) {
    out.push(
      active.playerId === viewerId
        ? t('a11y.board.read.yourTurn')
        : t('a11y.board.read.turn', { name: playerName(players, active.playerId) }),
    );
  }

  for (const f of state.view?.header ?? []) out.push(spokenFactText(f, players));

  if (state.standings?.length) {
    out.push(
      t('a11y.board.read.scores', {
        list: state.standings
          .map((s) => `${playerName(players, s.playerId)} ${scoreText(s)}`)
          .join(', '),
      }),
    );
  }

  // The table's own cards: piles, stacks and anything shared, nobody's.
  for (const z of zones) {
    if (z.ownerId) continue;
    if (z.kind === 'pile' || z.kind === 'stack') out.push(zoneSpokenLabel(z, players, viewerId));
    else if (z.kind === 'spread' && z.shared) {
      const cards = (z.groups ?? []).flatMap((g) => g.cards);
      const loose = (z.cards ?? []).filter((c) => !c.faceDown).map((c) => c.card);
      const all = [...loose, ...cards];
      out.push(
        all.length
          ? t('a11y.board.zone.cards', { zone: zoneName(z, players, viewerId), cards: cardsSpoken(all) })
          : t('a11y.board.pile.empty', { pile: zoneName(z, players, viewerId) }),
      );
    }
  }

  const mySpreads = zones.filter((z) => z.kind === 'spread' && z.ownerId === viewerId);
  if (mySpreads.length) {
    const melds = mySpreads.flatMap((z) => z.groups ?? []);
    out.push(
      melds.length
        ? t('a11y.board.read.yourMelds', { list: melds.map((g) => groupSpokenLabel(g)).join('; ') })
        : t('a11y.board.read.noMelds'),
    );
  }

  for (const hand of zones.filter((z) => z.kind === 'hand' && z.ownerId === viewerId)) {
    out.push(handSpokenLabel(label(hand.labelKey) || t('a11y.board.region.hand'), (hand.cards ?? []).length || hand.count));
  }

  return out.filter(Boolean).join('. ');
}

// ---------------------------------------------------------------------------
// Keyboard

/** What a key on the board means, before anything about the board is known. */
export type BoardShortcut = 'stack' | 'pile' | 'undo' | 'primary' | 'read' | 'moves' | 'help' | 'escape';

/**
 * Which shortcut a key press is, if any.
 *
 * Single keys only, and never with Ctrl, Cmd or Alt held: those belong to the
 * browser and the operating system, and a game that answered Ctrl+D would be
 * fighting the bookmark it was meant to make. Shift is allowed only where it
 * is part of the character ("?").
 */
export function shortcutForKey(e: {
  key: string;
  ctrlKey?: boolean;
  metaKey?: boolean;
  altKey?: boolean;
  shiftKey?: boolean;
}): BoardShortcut | null {
  if (e.ctrlKey || e.metaKey || e.altKey) return null;
  if (e.key === '?') return 'help';
  if (e.key === 'Escape') return 'escape';
  if (e.key === 'Enter') return e.shiftKey ? null : 'primary';
  if (e.shiftKey) return null;
  switch (e.key.toLowerCase()) {
    case 'd':
      return 'stack';
    case 't':
      return 'pile';
    case 'u':
      return 'undo';
    case 'r':
      return 'read';
    case 'm':
      return 'moves';
    default:
      return null;
  }
}

/** The letter shown in a tooltip for each shortcut that has one. */
export const SHORTCUT_KEYS: Partial<Record<BoardShortcut, string>> = {
  stack: 'D',
  pile: 'T',
  undo: 'U',
  read: 'R',
  moves: 'M',
  help: '?',
};

/**
 * Whether a key press belongs to whatever has focus rather than to the board:
 * typing into a field, or a control that has its own use for the key.
 */
export function isTypingTarget(el: { tagName?: string; isContentEditable?: boolean; getAttribute?: (n: string) => string | null } | null | undefined): boolean {
  if (!el) return false;
  const tag = (el.tagName ?? '').toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (el.isContentEditable) return true;
  const role = el.getAttribute?.('role');
  return role === 'textbox' || role === 'slider' || role === 'spinbutton';
}

/**
 * Which of the piles a player may take from is the one D and T mean.
 *
 * Read off the kinds the module gave the zones, never the game: a draw is
 * from a face-down `stack`, taking is from a face-up `pile`. Where the press
 * spots (`sourceSpotsFor`) name one of each, that is Žolíky's deck and
 * discard; where they name one stack, it is solitaire's stock.
 */
export function sourceSpotFor(kind: 'stack' | 'pile', spots: DropSpot[], zones: Zone[]): DropSpot | undefined {
  return spots.find((s) => zones.find((z) => `zone-${z.id}` === s.elementId)?.kind === kind);
}

/** The take-back on offer right now, if any. Declared by the module (`undo`), never guessed. */
export function undoOfferIn(offers: ActionOffer[]): ActionOffer | undefined {
  return offers.find((o) => o.enabled && o.undo);
}

/**
 * Whether sending this action ends the hand for the player who sends it, as
 * far as the client can tell without knowing any game's rules.
 *
 * There is no flag for it on the wire, so this is deliberately narrow — only
 * two signals that cannot be wrong about a game they were not written for:
 *
 * - the move plays every card the player holds from their hand, which in
 *   every game here is going out (a last discard, a meld of the whole hand);
 * - the module calls the verb `knock`, which is Gin Rummy's going down.
 *
 * Anything else — a showdown, a stand — is not asked about.
 */
export function endsHandFor(
  offer: ActionOffer | undefined,
  action: MatchAction,
  handZoneIds: ReadonlySet<string>,
  handCount: number,
): boolean {
  if (!offer) return false;
  if (offer.verb === 'knock') return true;
  const cards = action.cards ?? [];
  if (!cards.length || handCount <= 0) return false;
  // Cards that start on the table — a run lifted off a column, cards moved
  // between melds — are not the hand being emptied.
  if (offer.source?.meldId || offer.source?.zone === 'from_meld') return false;
  if (offer.source?.zoneId && !handZoneIds.has(offer.source.zoneId)) return false;
  return cards.length >= handCount;
}

/** The facts on an offer's own control, spoken: "Call, 40". */
export function offerSpokenName(title: string, facts: Fact[] | undefined, players: MatchPlayer[] = []): string {
  const extra = (facts ?? []).map((f) => spokenFactText(f, players)).filter(Boolean);
  return [title, ...extra].join(', ');
}

/**
 * What pressing an offer does, for its tooltip — read off the offer's shape,
 * never its game: a move that takes a card from a pile, one that plays the
 * picked cards somewhere, one that needs cards composed or an amount chosen,
 * a take-back. Undefined where the shape says nothing the label has not.
 */
export function offerTip(offer: ActionOffer, zones: Zone[], players: MatchPlayer[], viewerId: string): string | undefined {
  if (offer.undo) return t('a11y.board.tip.undo');
  const need = offer.source?.minCards ?? 0;
  const intParam = (offer.params ?? []).find((p) => p.kind === 'int' && !p.cards?.length);
  if (intParam) return t('a11y.board.tip.amount', { min: intParam.min ?? 0, max: intParam.max ?? intParam.min ?? 0 });
  if (offer.composite) return t('a11y.board.tip.compose', { n: need || 1 });
  if (need > 0) {
    const at = offer.target?.meldId ? `group-${offer.target.meldId}` : offer.target?.zoneId ? `zone-${offer.target.zoneId}` : undefined;
    if (at) return t('a11y.board.tip.playTo', { target: targetName(at, zones, players, viewerId) });
    return undefined;
  }
  const from = offer.source?.zoneId ? zones.find((z) => z.id === offer.source!.zoneId) : undefined;
  if (from && (from.kind === 'pile' || from.kind === 'stack')) {
    return t('a11y.board.tip.takeFrom', { pile: zoneName(from, players, viewerId) });
  }
  return undefined;
}
