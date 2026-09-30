import type { ActionOffer } from '@/src/api/matchTypes';
import { refusalFor } from '@/src/components/match/OfferBar';

/**
 * A press on a control that cannot go opens a sheet saying why, and the sheet
 * is only as good as what this hands it. The one outcome it must never have is
 * an empty answer: that is the silent press the sheet replaced.
 */
describe('refusalFor', () => {
  const base: ActionOffer = { id: 'x', verb: 'do_thing', enabled: true };

  it("carries the engine's own refusal, rules and remedy with it", () => {
    const offer: ActionOffer = {
      ...base,
      enabled: false,
      whyNot: 'NOT_YOUR_TURN',
      ruleIds: ['rule.turns'],
      remedyOfferId: 'y',
    };
    expect(refusalFor(offer, [], undefined)).toEqual({
      code: 'NOT_YOUR_TURN',
      ruleIds: ['rule.turns'],
      remedy: undefined,
      remedyOfferId: 'y',
    });
  });

  it('still says something when the engine gave no reason', () => {
    expect(refusalFor({ ...base, enabled: false }, [], undefined)).toEqual({ labelKey: 'why.unavailable' });
  });

  it('asks for cards when an offer needs some and none are picked', () => {
    const offer: ActionOffer = { ...base, source: { zone: 'hand', minCards: 1, maxCards: 1 } };
    expect(refusalFor(offer, [], undefined).labelKey).toBeTruthy();
  });

  it('tells a combination control how many cards to pick', () => {
    const offer: ActionOffer = { ...base, composite: true, source: { zone: 'hand', minCards: 3, maxCards: 7 } };
    expect(refusalFor(offer, [], undefined)).toEqual({ labelKey: 'why.pickAtLeast', params: { n: 3 } });
  });

  it('has nothing to explain about an offer that is ready', () => {
    expect(refusalFor(base, [], undefined)).toEqual({});
  });
});
