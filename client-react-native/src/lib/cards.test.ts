import { cardSuit, displayRank, parseCard } from '@/src/lib/cards';

// What is left of this file after the bespoke Žolíky screen went.
//
// `autoOrganizeHand` and `moveCardToIndex` were tested here too — sorting a
// rummy hand into runs and sets, and reordering it by drag. Both were rules
// living in a client, and both went with the screen that needed them. Drawing
// a card is not: every game draws "TD" the same way.

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
