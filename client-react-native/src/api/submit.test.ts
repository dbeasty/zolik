import { isOneTap, submissionFor, type ActionOffer } from './matchTypes';

/**
 * The Canasta meld offer for a hand holding four queens, as the server sends
 * it: press it and all four go down, but a player may pick any three of them.
 * Before `submit` those were the same number and the smaller meld was
 * unreachable — the zone stayed dark until the fourth queen was picked.
 */
const fourQueens: ActionOffer = {
  id: 'lay_meld:Q',
  verb: 'lay_meld',
  enabled: true,
  source: {
    zone: 'hand',
    ownerId: 'p1',
    zoneId: 'hand:p1',
    cards: ['QC', 'QD', 'QH', 'QS'],
    submit: ['QC', 'QD', 'QH', 'QS'],
    minCards: 3,
    maxCards: 4,
  },
  target: { zone: 'table', zoneId: 'melds:t0' },
};

describe('an offer that names its own combination', () => {
  it('is still a button, though its minimum is shorter than its list', () => {
    expect(isOneTap(fourQueens)).toBe(true);
  });

  it('sends the whole combination when nobody chose', () => {
    expect(submissionFor(fourQueens)).toEqual({
      offerId: 'lay_meld:Q',
      verb: 'lay_meld',
      cards: ['QC', 'QD', 'QH', 'QS'],
    });
  });

  it('sends what was chosen when somebody did choose', () => {
    expect(submissionFor(fourQueens, { cards: ['QC', 'QD', 'QH'] })).toMatchObject({
      cards: ['QC', 'QD', 'QH'],
    });
  });

  it('still refuses a choice outside the offer’s own bounds', () => {
    expect(submissionFor(fourQueens, { cards: ['QC', 'QD'] })).toBeNull();
  });
});

describe('an offer that names no combination', () => {
  // Every single-card offer in every module, unchanged: the submission is the
  // list when the list is exactly as long as the minimum.
  const oneOfMany: ActionOffer = {
    id: 'lay_off:t0-7',
    verb: 'lay_off',
    enabled: true,
    source: { zone: 'hand', ownerId: 'p1', zoneId: 'hand:p1', cards: ['7D', '7H'], minCards: 1, maxCards: 4 },
    target: { zone: 'meld', ownerId: 'p1', meldId: 't0-7', zoneId: 'melds:t0' },
  };

  it('is not a button, because which card goes is a person’s choice', () => {
    expect(isOneTap(oneOfMany)).toBe(false);
  });

  it('describes no submission on its own', () => {
    expect(submissionFor(oneOfMany)).toBeNull();
  });
});
