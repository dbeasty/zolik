import { AccessibilityInfo } from 'react-native';

import { announce, announceGame, resetAnnouncer } from '@/src/a11y/announce';
import { cardSpokenName, cardsSpoken } from '@/src/a11y/cardNames';
import { DEFAULT_A11Y_PREFS, parsePrefs, setA11yPref } from '@/src/a11y/prefs';
import { setActiveDeck } from '@/src/lib/deck';
import { setLocale } from '@/src/lib/i18n';

afterEach(() => {
  setActiveDeck('french');
  setLocale('en');
});

describe('cardSpokenName', () => {
  it('names a French card in words, not its code or glyph', () => {
    expect(cardSpokenName('KH')).toBe('King of Hearts');
    expect(cardSpokenName('TD')).toBe('Ten of Diamonds');
    expect(cardSpokenName('AS')).toBe('Ace of Spades');
    expect(cardSpokenName('JOKER1')).toBe('Joker');
  });

  it('names the German pack the way its faces are drawn', () => {
    setActiveDeck('german');
    expect(cardSpokenName('QS')).toBe('Over of Leaves');
    expect(cardSpokenName('JD')).toBe('Under of Bells');
    expect(cardSpokenName('KH')).toBe('King of Hearts');
  });

  it('names Last Card and tiles by colour', () => {
    setActiveDeck('lastcard');
    expect(cardSpokenName('W4')).toBe('Wild Draw Four');
    expect(cardSpokenName('7-R')).toBe('Red 7');
  });

  it('leaves anything that is not a card alone', () => {
    expect(cardSpokenName('Anna')).toBe('Anna');
    expect(cardsSpoken(['5H', '6H'])).toBe('Five of Hearts, Six of Hearts');
  });

  it('follows the language', () => {
    setLocale('cs');
    expect(cardSpokenName('KH')).not.toBe('King of Hearts');
  });
});

describe('parsePrefs', () => {
  it('defaults everything to the board as it always was', () => {
    expect(parsePrefs(null)).toEqual(DEFAULT_A11Y_PREFS);
    expect(parsePrefs('not json')).toEqual(DEFAULT_A11Y_PREFS);
  });

  it('keeps good fields and drops malformed ones', () => {
    const p = parsePrefs(JSON.stringify({ cardSize: 'large', announce: 'loud', fourColour: true, tooltips: 'yes' }));
    expect(p.cardSize).toBe('large');
    expect(p.announce).toBe('all');
    expect(p.fourColour).toBe(true);
    expect(p.tooltips).toBe(true);
  });
});

describe('announceGame', () => {
  const spy = jest.spyOn(AccessibilityInfo, 'announceForAccessibility').mockImplementation(() => {});
  beforeEach(() => {
    jest.useFakeTimers();
    resetAnnouncer();
    spy.mockClear();
  });
  afterEach(() => jest.useRealTimers());

  it('drops other players moves when only my turn is wanted', async () => {
    await setA11yPref('announce', 'mine');
    announceGame('Anna drew a card', 'move');
    announceGame('Your turn', 'turn');
    jest.runAllTimers();
    expect(spy.mock.calls.map((c) => c[0])).toEqual(['Your turn']);
    await setA11yPref('announce', 'all');
  });

  it('says nothing when off', async () => {
    await setA11yPref('announce', 'off');
    announceGame('Your turn', 'turn');
    jest.runAllTimers();
    expect(spy).not.toHaveBeenCalled();
    await setA11yPref('announce', 'all');
  });

  it('does not repeat itself, and spaces a burst out', () => {
    announce('Anna drew a card');
    announce('Anna drew a card');
    announce('Ben drew a card');
    expect(spy).toHaveBeenCalledTimes(1);
    jest.runAllTimers();
    expect(spy.mock.calls.map((c) => c[0])).toEqual(['Anna drew a card', 'Ben drew a card']);
  });
});
