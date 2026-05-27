import { cardSuit, displayRank, parseCard } from '@/src/lib/cards';

describe('cards', () => {
  it('parses standard cards', () => {
    const d = parseCard('KH');
    expect(d.rank).toBe('K');
    expect(d.suit).toBe('H');
    expect(d.isRed).toBe(true);
    expect(d.isJoker).toBe(false);
  });

  it('parses ten', () => {
    expect(displayRank('TH')).toBe('10');
    expect(cardSuit('TH')).toBe('H');
  });

  it('parses joker', () => {
    const d = parseCard('JOKER1');
    expect(d.isJoker).toBe(true);
    expect(d.rank).toBe('JKR');
  });
});
