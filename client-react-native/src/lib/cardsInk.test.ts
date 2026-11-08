import { lastCardColourOfKey, lastCardInkRuns } from '@/src/lib/cards';

describe('lastCardInkRuns', () => {
  it('colours each card and colour a line names, and nothing else', () => {
    expect(lastCardInkRuns('Handy Honza played ■ Draw Two — Brisk Barbora draws 2 and misses a turn')).toEqual([
      { text: 'Handy Honza played ' },
      { text: '■ Draw Two', colour: 'A' },
      { text: ' — Brisk Barbora draws 2 and misses a turn' },
    ]);
    expect(lastCardInkRuns('Plucky Petra played ■4')).toEqual([
      { text: 'Plucky Petra played ' },
      { text: '■4', colour: 'A' },
    ]);
    expect(lastCardInkRuns('Ada played Wild — the colour is now ◆ Teal')).toEqual([
      { text: 'Ada played Wild — the colour is now ' },
      { text: '◆ Teal', colour: 'T' },
    ]);
  });

  it('leaves a line with no shapes in it whole', () => {
    expect(lastCardInkRuns('Steady Stanislav drew a card')).toEqual([{ text: 'Steady Stanislav drew a card' }]);
  });
});

describe('lastCardColourOfKey', () => {
  it('reads only the colour keys', () => {
    expect(lastCardColourOfKey('lastcard.colour.V')).toBe('V');
    expect(lastCardColourOfKey('lastcard.colourName.V')).toBeUndefined();
    expect(lastCardColourOfKey(undefined)).toBeUndefined();
  });
});

describe('frenchSuitOfKey', () => {
  it('reads the suit a key names, and nothing else', () => {
    const { frenchSuitOfKey } = require('@/src/lib/cards');
    expect(frenchSuitOfKey('suit.H')).toBe('H');
    expect(frenchSuitOfKey('suit.S')).toBe('S');
    expect(frenchSuitOfKey('suit.german.H')).toBeUndefined();
    expect(frenchSuitOfKey('lastcard.colour.V')).toBeUndefined();
    expect(frenchSuitOfKey(undefined)).toBeUndefined();
  });
});
