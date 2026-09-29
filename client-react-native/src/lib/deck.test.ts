import { cardText } from '@/src/lib/cards';
import { germanIndex, setActiveDeck } from '@/src/lib/deck';

describe('the German pack', () => {
  afterEach(() => setActiveDeck('french'));

  it('names the courts by the German letters', () => {
    expect(['7', '10', 'J', 'Q', 'K', 'A'].map(germanIndex)).toEqual(['7', '10', 'U', 'O', 'K', 'A']);
  });

  it('names a card in a sentence the way its face is drawn', () => {
    setActiveDeck('german');
    expect(cardText('QS')).toBe('O🍃');
    expect(cardText('TD')).toBe('10🔔');
    expect(cardText('AH')).toBe('A♥');
  });

  it('leaves every other table French', () => {
    expect(cardText('QS')).toBe('Q♠');
  });
});
