import { defaultParam, submissionFor, type ActionOffer, type ParamSpec } from '@/src/api/matchTypes';

const choices = [
  { value: 'C', labelKey: 'a' },
  { value: 'T', labelKey: 'b' },
  { value: 'A', labelKey: 'c' },
];

describe('a choice parameter’s starting value', () => {
  test('is the server’s default choice where it names one', () => {
    expect(defaultParam({ name: 'colour', labelKey: 'k', choices, defaultChoice: 'A' })).toBe('A');
  });

  test('is the first choice where it names none, or one that is not offered', () => {
    expect(defaultParam({ name: 'colour', labelKey: 'k', choices })).toBe('C');
    expect(defaultParam({ name: 'colour', labelKey: 'k', choices, defaultChoice: 'X' })).toBe('C');
  });

  test('is what a press sends when the player picked nothing', () => {
    const spec: ParamSpec = { name: 'colour', labelKey: 'k', choices, defaultChoice: 'T' };
    const offer: ActionOffer = {
      id: 'play_card',
      verb: 'play_card',
      enabled: true,
      source: { zone: 'hand', cards: ['W'], minCards: 1, maxCards: 1 },
      params: [spec],
    };
    expect(submissionFor(offer, { cards: ['W'] })?.params).toEqual({ colour: 'T' });
  });
});
