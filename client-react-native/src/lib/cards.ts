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

// moveCard moves the card at `from` so it lands immediately before the card
// currently at `to` (in the original array's indexing).
export function moveCard(cards: string[], from: number, to: number): string[] {
  if (from === to) return cards;
  const copy = [...cards];
  const [item] = copy.splice(from, 1);
  const insertAt = from < to ? to - 1 : to;
  copy.splice(insertAt, 0, item);
  return copy;
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
