import type { ActionOffer } from '@/src/api/matchTypes';
import { leadOffer } from '@/src/components/match/OfferBar';

/**
 * Prší's Play also asks a suit. A figure to send does not stand in for the
 * cards to send it with, so the control waits for a card rather than lighting
 * up and sending nothing when pressed.
 */
describe('an offer that takes cards and a parameter', () => {
  const play = {
    id: 'play_card',
    verb: 'play_card',
    enabled: true,
    source: { zoneId: 'hand:me', minCards: 1, maxCards: 1, cards: ['TC', '9S'] },
    params: [{ name: 'suit', kind: 'choice', choices: [{ value: 'H' }, { value: 'S' }] }],
  } as unknown as ActionOffer;

  it('is not ready with nothing selected', () => {
    expect(leadOffer([play], [])).toBeUndefined();
  });

  it('is ready once a card that fits is selected', () => {
    expect(leadOffer([play], ['TC'])?.id).toBe('play_card');
  });

  it('is not ready with a card it cannot take', () => {
    expect(leadOffer([play], ['KH'])).toBeUndefined();
  });

  it('still needs no card when the offer takes none: a raise', () => {
    const raise = { ...play, id: 'raise', source: undefined } as ActionOffer;
    expect(leadOffer([raise], [])?.id).toBe('raise');
  });
});
