import { choicesToAsk, defaultParam, submissionFor, type ActionOffer, type ParamSpec } from '@/src/api/matchTypes';

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

describe('the questions a move asks before it goes', () => {
  const colour: ParamSpec = { name: 'colour', labelKey: 'k', choices, cards: ['W', 'W4'] };
  const offer: ActionOffer = {
    id: 'play_card',
    verb: 'play_card',
    enabled: true,
    source: { zone: 'hand', cards: ['W', 'C-7'], minCards: 1, maxCards: 1 },
    params: [colour],
  };

  test('a card the question is for asks it, whichever way it was played', () => {
    const { ask } = choicesToAsk(offer, { offerId: 'play_card', verb: 'play_card', cards: ['W'], params: { colour: 'C' } });
    expect(ask.map((p) => p.name)).toEqual(['colour']);
  });

  test('a card it is not for asks nothing, and sends no answer', () => {
    const { ask, action } = choicesToAsk(offer, {
      offerId: 'play_card',
      verb: 'play_card',
      cards: ['C-7'],
      params: { colour: 'C' },
    });
    expect(ask).toEqual([]);
    expect(action.params).toBeUndefined();
  });

  test('a question for every card is never asked here — its control sets it', () => {
    const always: ActionOffer = { ...offer, params: [{ name: 'n', labelKey: 'k', choices }] };
    const { ask, action } = choicesToAsk(always, { offerId: 'play_card', verb: 'play_card', cards: ['W'], params: { n: 'T' } });
    expect(ask).toEqual([]);
    expect(action.params).toEqual({ n: 'T' });
  });
});
