/**
 * How a card string is drawn.
 *
 * Presentation only, and that is the whole file now. It used to also hold
 * `autoOrganizeHand`, `contractLabel`, `rulesSummaryLines` and
 * `profileDisplayName` — sorting a rummy hand, naming a rummy contract,
 * re-typing each rummy profile's constants — every one of which was a rule
 * living in a client. They went with the screen that needed them.
 *
 * What is left knows that "TD" is the ten of diamonds and that diamonds are
 * red. That is a fact about a deck, not about a game: Žolíky, Canasta and
 * Hold'em all draw the same card the same way, which is why one CardView can
 * serve all of them.
 */

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
  // "TD" is a ten, so its suit is the second character rather than the last —
  // which is the same thing for a two-character card and matters only because
  // a rank could one day be two characters.
  if (card[0] === 'T') return card[1];
  return card[card.length - 1];
}
