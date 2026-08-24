import type { Contract, ResolvedRules } from '@/src/api/types';
import { countLabel, t } from '@/src/lib/i18n';

export type CardDisplay = {
  rank: string;
  suitSymbol: string;
  suit: string;
  isRed: boolean;
  isJoker: boolean;
};

const SUIT_SYMBOLS: Record<string, string> = {
  H: '♥',
  D: '♦',
  C: '♣',
  S: '♠',
};

export function parseCard(card: string): CardDisplay {
  if (card.startsWith('JOKER')) {
    return {
      rank: 'JKR',
      suitSymbol: '★',
      suit: '',
      isRed: false,
      isJoker: true,
    };
  }
  const suit = cardSuit(card);
  return {
    rank: displayRank(card),
    suitSymbol: SUIT_SYMBOLS[suit] ?? '?',
    suit,
    isRed: suit === 'H' || suit === 'D',
    isJoker: false,
  };
}

export function displayRank(card: string): string {
  if (card.startsWith('JOKER')) return 'JKR';
  if (!card.length) return '?';
  if (card[0] === 'T') return '10';
  return card[0];
}

export function cardSuit(card: string): string {
  if (card.length < 2) return 'S';
  if (card[0] === 'T') return card[1];
  return card[card.length - 1];
}

function rankOrder(card: string): number {
  if (card.startsWith('JOKER')) return 100;
  const r = displayRank(card);
  const order: Record<string, number> = {
    A: 1,
    '2': 2,
    '3': 3,
    '4': 4,
    '5': 5,
    '6': 6,
    '7': 7,
    '8': 8,
    '9': 9,
    '10': 10,
    J: 11,
    Q: 12,
    K: 13,
  };
  return order[r] ?? 50;
}

// autoOrganizeHand arranges a hand low-to-high while keeping potential melds
// visually contiguous: same-rank duplicates (set material) are clustered
// together, and same-suit consecutive-rank runs are clustered together, with
// each cluster placed by its lowest rank. Jokers are wild and don't belong to
// any one cluster, so they're kept together at the end.
export function autoOrganizeHand(cards: string[]): string[] {
  const jokers = cards.filter((c) => c.startsWith('JOKER'));
  const nonJokers = cards.filter((c) => !c.startsWith('JOKER'));

  const rankCounts = new Map<string, number>();
  for (const c of nonJokers) {
    const r = displayRank(c);
    rankCounts.set(r, (rankCounts.get(r) ?? 0) + 1);
  }

  const multiples: string[] = [];
  const singles: string[] = [];
  for (const c of nonJokers) {
    const r = displayRank(c);
    if ((rankCounts.get(r) ?? 0) >= 2) {
      multiples.push(c);
    } else {
      singles.push(c);
    }
  }

  const multipleGroups = new Map<string, string[]>();
  for (const c of multiples) {
    const r = displayRank(c);
    const arr = multipleGroups.get(r) ?? [];
    arr.push(c);
    multipleGroups.set(r, arr);
  }
  for (const arr of multipleGroups.values()) {
    arr.sort((a, b) => cardSuit(a).localeCompare(cardSuit(b)));
  }

  const bySuit = new Map<string, string[]>();
  for (const c of singles) {
    const s = cardSuit(c);
    const arr = bySuit.get(s) ?? [];
    arr.push(c);
    bySuit.set(s, arr);
  }

  type Group = { key: number; cards: string[] };
  const groups: Group[] = [];

  for (const arr of bySuit.values()) {
    arr.sort((a, b) => rankOrder(a) - rankOrder(b));
    let run: string[] = [];
    const flush = () => {
      if (run.length === 0) return;
      groups.push({ key: rankOrder(run[0]), cards: run });
      run = [];
    };
    for (const c of arr) {
      if (run.length === 0 || rankOrder(c) === rankOrder(run[run.length - 1]) + 1) {
        run.push(c);
      } else {
        flush();
        run.push(c);
      }
    }
    flush();
  }

  for (const arr of multipleGroups.values()) {
    groups.push({ key: rankOrder(arr[0]), cards: arr });
  }

  groups.sort((a, b) => a.key - b.key);

  return [...groups.flatMap((g) => g.cards), ...jokers];
}

function isJoker(card: string): boolean {
  return card.startsWith('JOKER');
}

// insertCardIntoHand slots a newly arrived card (a draw, or a card handed back
// after a rolled-back lay) into the place it belongs in a hand the player has
// already arranged, instead of dropping it at one edge and leaving them to
// drag it home. It only moves the one card: everything already in the hand
// keeps its relative order, so a manual arrangement survives a draw.
//
// "Where it belongs" is decided by the same rules autoOrganizeHand uses —
// same-rank cards cluster (and outrank a run, exactly as they do there), a
// same-suit neighbour rank extends the run, jokers go to the tail — so that
// hitting Auto-organize right after a draw doesn't shuffle the card somewhere
// else again. Anything related to nothing lands at its own rank.
export function insertCardIntoHand(hand: string[], card: string): string[] {
  const out = [...hand];
  out.splice(handInsertIndex(hand, card), 0, card);
  return out;
}

function handInsertIndex(hand: string[], card: string): number {
  if (isJoker(card)) return hand.length;
  // Jokers sit at the tail and belong to no cluster, so nothing is ever
  // placed past them.
  const firstJoker = hand.findIndex(isJoker);
  const end = firstJoker === -1 ? hand.length : firstJoker;

  const rank = rankOrder(card);
  const suit = cardSuit(card);

  const sameRank: number[] = [];
  for (let i = 0; i < end; i++) {
    if (!isJoker(hand[i]) && rankOrder(hand[i]) === rank) sameRank.push(i);
  }

  // A same-rank partner wins over a run the card could extend, because that
  // is the call autoOrganizeHand makes too (any rank held twice becomes a
  // cluster, and singles alone are gathered into runs). Deciding it
  // differently here would mean the card visibly hopped the moment the player
  // hit Auto-organize.
  if (sameRank.length > 0) {
    // Into the cluster, in the alphabetical-by-suit order autoOrganizeHand
    // sorts a cluster into.
    const before = sameRank.find((i) => cardSuit(hand[i]).localeCompare(suit) > 0);
    return before ?? sameRank[sameRank.length - 1] + 1;
  }

  // Runs read low-to-high left-to-right, so sit just past the card one rank
  // below (the last copy of it, with two decks in play) or just before the
  // card one rank above.
  for (let i = end - 1; i >= 0; i--) {
    if (!isJoker(hand[i]) && cardSuit(hand[i]) === suit && rankOrder(hand[i]) === rank - 1) {
      return i + 1;
    }
  }
  for (let i = 0; i < end; i++) {
    if (!isJoker(hand[i]) && cardSuit(hand[i]) === suit && rankOrder(hand[i]) === rank + 1) {
      return i;
    }
  }

  // Related to nothing in hand: fall in at its own rank. No card of this rank
  // is in the hand at all here (that would have been the cluster case), so
  // this can't cut a cluster or a run in half.
  for (let i = 0; i < end; i++) {
    if (!isJoker(hand[i]) && rankOrder(hand[i]) > rank) return i;
  }
  return end;
}

// moveCardToIndex moves the card at `from` so it ends up sitting at index
// `to` in the resulting array (i.e. `to` is a final position, not an
// "insert before" reference) — the semantics a drag gesture wants: drop a
// card N slots over and it lands in slot N.
export function moveCardToIndex(cards: string[], from: number, to: number): string[] {
  if (from === to) return cards;
  const copy = [...cards];
  const [item] = copy.splice(from, 1);
  const clamped = Math.max(0, Math.min(copy.length, to));
  copy.splice(clamped, 0, item);
  return copy;
}

/**
 * Describes a contract the *server* resolved, rather than looking one up by
 * deal number. The deal-to-contract table used to live here, duplicating
 * server/internal/rules/profiles.go; a profile with a different rotation
 * silently rendered the wrong requirement. Now the shape arrives on the wire
 * and this only puts words around it.
 */
export function contractLabel(contract: Contract | undefined): string {
  if (!contract) return '';
  const parts: string[] = [];
  if (contract.sets > 0) parts.push(countLabel(contract.sets, 'sets'));
  if (contract.runs > 0) parts.push(countLabel(contract.runs, 'runs'));
  if (parts.length === 0) {
    return contract.requireCleanRun ? t('contract.cleanRunOnly') : t('contract.any');
  }
  const base = parts.join(', ');
  return contract.requireCleanRun ? t('contract.cleanRunSuffix', { base }) : base;
}

/**
 * Header label for the current deal.
 *
 * A fixed-length match (`fixedDealCount > 0`) counts its deals and names the
 * contract for this one; a score-limited match just re-deals until someone
 * crosses the target, so naming a deal count there would be wrong. Both facts
 * come from the ruleset the server sent — no profile name is consulted, which
 * is what lets a third profile render correctly with no client change.
 */
export function dealHeaderLabel(
  rules: ResolvedRules | undefined,
  contract: Contract | undefined,
  game: number,
): string {
  if (!rules || rules.fixedDealCount <= 0) {
    return t('header.deal', { n: game });
  }
  const contractText = contractLabel(contract);
  const params = { n: game, total: rules.fixedDealCount, contract: contractText };
  return contractText ? t('header.gameOfWithContract', params) : t('header.gameOf', params);
}

/** Short human name for a rules profile, for display next to the deal header. */
export function profileDisplayName(rulesProfile: string | undefined): string {
  if (rulesProfile === 'zolik_classic') return 'Žolík Classic';
  if (rulesProfile === 'continental') return 'Continental Rummy';
  return rulesProfile ? 'Custom house rules' : 'Continental Rummy';
}

/**
 * Full rule breakdown for the in-game "Rules" panel.
 *
 * Every value below is read off the ruleset the server resolved for *this*
 * game. It used to be a second copy of server/internal/rules/profiles.go
 * keyed on the profile name, which meant a house-rule override or a new
 * profile displayed one thing while the engine enforced another.
 */
export function rulesSummaryLines(
  rules: ResolvedRules | undefined,
  contract: Contract | undefined,
): { label: string; value: string }[] {
  if (!rules) return [];
  const lines = [
    { label: t('rules.variation'), value: profileDisplayName(rules.profile) },
    { label: t('rules.dealSize'), value: t('rules.cards', { n: rules.dealSize }) },
    { label: t('rules.minSetSize'), value: t('rules.cards', { n: rules.minSetSize }) },
    { label: t('rules.minRunSize'), value: t('rules.cards', { n: rules.minRunSize }) },
    { label: t('rules.toGoDown'), value: contractLabel(contract) || t('contract.any') },
    {
      label: t('rules.meldFloor'),
      value:
        rules.initialMeldMinimum > 0
          ? t('rules.floorPoints', { n: rules.initialMeldMinimum })
          : t('rules.floorOff'),
    },
    { label: t('rules.discardPickup'), value: discardPickupSummary(rules) },
    {
      label: t('rules.jokers'),
      value: rules.jokerDiscardRestricted
        ? t('rules.jokersRestricted')
        : t('rules.jokersFree'),
    },
    { label: t('rules.matchEnds'), value: matchEndSummary(rules) },
  ];
  return lines;
}

function discardPickupSummary(rules: ResolvedRules): string {
  const scope =
    rules.discardPickupMode === 'any_from_pile'
      ? t('rules.pickupAnyFromPile')
      : t('rules.pickupTopOnly');
  return rules.discardDrawMinRound > 1
    ? t('rules.pickupLocked', { scope, n: rules.discardDrawMinRound })
    : scope;
}

function matchEndSummary(rules: ResolvedRules): string {
  if (rules.matchEndMode === 'at_score') {
    return t('rules.endsAtScore', { n: rules.targetScore });
  }
  return rules.fixedDealCount > 0
    ? t('rules.endsAfterDeals', { n: rules.fixedDealCount })
    : t('rules.endsOnDeal');
}
