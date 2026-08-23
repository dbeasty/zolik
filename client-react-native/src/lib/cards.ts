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

export function rankOrder(card: string): number {
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

// Client-side mirror of the server's ValidateMeld (server/internal/rules/meld.go)
// used only to decide when a staged group already reads as a complete meld —
// e.g. to auto-open the next staging box — never to gate what's actually sent
// to lay_meld, which the server validates for real. So it deliberately skips
// the server's rarer edge cases (adjacent-wild runs, ace-bridge windows):
// getting those wrong here just means the box doesn't auto-open a beat early,
// not a rules violation.
export function isViableMeld(cards: string[], minRunLength: number): boolean {
  if (cards.length < 3) return false;
  return isViableSet(cards) || isViableRun(cards, minRunLength);
}

function isViableSet(cards: string[]): boolean {
  const jokers = cards.filter((c) => c.startsWith('JOKER'));
  const naturals = cards.filter((c) => !c.startsWith('JOKER'));
  if (naturals.length === 0 || jokers.length > naturals.length) return false;
  const rank = displayRank(naturals[0]);
  if (!naturals.every((c) => displayRank(c) === rank)) return false;
  const suits = naturals.map(cardSuit);
  return new Set(suits).size === suits.length;
}

function isViableRun(cards: string[], minRunLength: number): boolean {
  if (cards.length < minRunLength) return false;
  const jokers = cards.filter((c) => c.startsWith('JOKER'));
  const naturals = cards.filter((c) => !c.startsWith('JOKER'));
  if (naturals.length === 0 || jokers.length > naturals.length) return false;
  const suits = new Set(naturals.map(cardSuit));
  if (suits.size !== 1) return false;
  const hasAce = naturals.some((c) => displayRank(c) === 'A');
  // Aces are flex — low (rankOrder's 1) or high (14) — same as the server's
  // run resolution, so e.g. Q-K-A is recognized alongside A-2-3.
  const aceValues = hasAce ? [1, 14] : [1];
  for (const aceValue of aceValues) {
    const ranks = naturals.map((c) => (displayRank(c) === 'A' ? aceValue : rankOrder(c)));
    if (new Set(ranks).size !== ranks.length) continue;
    const span = Math.max(...ranks) - Math.min(...ranks) + 1;
    // Every card in the group has to be part of the run — the jokers must
    // exactly fill the gaps within the naturals' span, no leftovers.
    if (span === cards.length) return true;
  }
  return false;
}

export function roundRequirementLabel(round: number): string {
  const labels = [
    '',
    'Two sets',
    'One set, one run',
    'Two runs',
    'Three sets',
    'Two sets, one run',
    'One set, two runs',
    'Three runs',
  ];
  return labels[round] ?? `Round ${round}`;
}

/**
 * Header label for the current deal. Continental has a fixed per-deal
 * contract (roundRequirementLabel); Žolík Classic and any other non-rotating
 * profile has no such contract — going down just requires one joker-free
 * run — so the header skips the contract phrase entirely.
 */
export function dealHeaderLabel(rulesProfile: string | undefined, game: number): string {
  if (rulesProfile === 'zolik_classic') {
    return `Deal ${game}`;
  }
  return `Game ${game}: ${roundRequirementLabel(game)}`;
}

/** Short human name for a rules profile, for display next to the deal header. */
export function profileDisplayName(rulesProfile: string | undefined): string {
  if (rulesProfile === 'zolik_classic') return 'Žolík Classic';
  if (rulesProfile === 'continental') return 'Continental Rummy';
  return rulesProfile ? 'Custom house rules' : 'Continental Rummy';
}

/**
 * Full rule breakdown for the in-game "Rules" panel. Mirrors the base
 * profile constants in server/internal/rules/profiles.go (deal size, run/set
 * minimums, discard pickup mode, joker rule) plus this table's live
 * overrides (meld-value floor, discard-lock round) which the server already
 * sends per game.
 */
export function rulesSummaryLines(
  rulesProfile: string | undefined,
  game: number,
  initialMeldMinimum: number,
  discardDrawMinRound: number,
): { label: string; value: string }[] {
  const isZolik = rulesProfile === 'zolik_classic';
  const lines = [
    { label: 'Variation', value: profileDisplayName(rulesProfile) },
    { label: 'Deal size', value: isZolik ? '13 cards' : '12 cards' },
    { label: 'Minimum run length', value: isZolik ? '3 cards' : '4 cards' },
    {
      label: 'To go down',
      value: isZolik
        ? 'Any mix of sets/runs, including all runs — at least one run must be joker-free'
        : roundRequirementLabel(game),
    },
    {
      label: 'Meld value floor',
      value: initialMeldMinimum > 0 ? `${initialMeldMinimum}+ points on your first meld(s)` : 'Off',
    },
    {
      label: 'Discard pickup',
      value:
        discardDrawMinRound > 1
          ? `Top card only, locked until round ${discardDrawMinRound}`
          : isZolik
            ? 'Any card in the pile (and everything stacked above it)'
            : 'Top card only',
    },
    { label: 'Jokers', value: 'Can never be discarded, except as the card that ends your hand' },
  ];
  return lines;
}
