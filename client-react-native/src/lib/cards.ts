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

export function sortHand(cards: string[], mode: 'rank' | 'suit'): string[] {
  const copy = [...cards];
  copy.sort((a, b) => {
    if (mode === 'suit') {
      const sa = cardSuit(a);
      const sb = cardSuit(b);
      if (sa !== sb) return sa.localeCompare(sb);
    }
    const ra = rankOrder(a);
    const rb = rankOrder(b);
    return ra - rb;
  });
  return copy;
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
