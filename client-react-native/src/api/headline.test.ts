import { offerHeadline, submissionFor, type ActionOffer } from './matchTypes';

/**
 * Hold'em's raise as the server sends it: a range from the minimum legal raise
 * to the whole stack, with the quick choices beside it, and the amount marked
 * as the thing the button sends. The bug this guards: the button read "Raise"
 * whatever the slider or "½ Pot" had set, so nothing on it said 483.
 */
const raise: ActionOffer = {
  id: 'raise',
  verb: 'raise',
  enabled: true,
  params: [
    {
      name: 'amount',
      kind: 'int',
      labelKey: 'holdem.prompt.raiseTo',
      min: 40,
      max: 1000,
      step: 1,
      default: 40,
      headline: true,
      choices: [
        { value: '105', labelKey: 'holdem.quick.halfPot' },
        { value: '170', labelKey: 'holdem.quick.pot' },
        { value: '1000', labelKey: 'holdem.quick.allIn' },
      ],
    },
  ],
};

describe('an offer whose parameter is its headline', () => {
  it('names the default before the player touches anything', () => {
    expect(offerHeadline(raise)).toEqual({ labelKey: 'holdem.prompt.raiseTo', value: '40' });
  });

  it('follows the value the slider or stepper set', () => {
    expect(offerHeadline(raise, { amount: '483' })?.value).toBe('483');
  });

  it('follows a quick choice', () => {
    const pot = raise.params![0].choices!.find((c) => c.labelKey === 'holdem.quick.pot')!;
    expect(offerHeadline(raise, { amount: pot.value })?.value).toBe('170');
  });

  it('clamps a figure typed past the range, as the stepper does', () => {
    expect(offerHeadline(raise, { amount: '5000' })?.value).toBe('1000');
    expect(offerHeadline(raise, { amount: '3' })?.value).toBe('40');
  });

  it('names exactly the figure the press sends', () => {
    // The title and the submission read the same in-progress value; if they
    // ever drift, the button is lying about what it does.
    for (const amount of [undefined, '40', '105', '483', '1000']) {
      const chosen = amount === undefined ? undefined : { amount };
      const sent = submissionFor(raise, { params: chosen })?.params?.amount;
      expect(offerHeadline(raise, chosen)?.value).toBe(sent);
    }
  });
});

describe('an offer with no headline parameter', () => {
  it('keeps its verb — a tile position is not what a button does', () => {
    const split: ActionOffer = {
      id: 'split',
      verb: 'split',
      enabled: true,
      params: [{ name: 'position', kind: 'int', labelKey: 'rummytiles.param.position', min: 3, max: 6 }],
    };
    expect(offerHeadline(split, { position: '4' })).toBeUndefined();
  });

  it('keeps its verb when it has no parameters at all', () => {
    expect(offerHeadline({ id: 'fold', verb: 'fold', enabled: true })).toBeUndefined();
  });
});

describe('a value chosen against an older range', () => {
  // The in-progress store outlives the offer it was dialled against: set 900,
  // leave it unsent, and the next street arrives with only 600 behind it. The
  // control shows 600; the press has to send 600, not a figure the engine will
  // refuse and the button never showed.
  const shorter: ActionOffer = {
    ...raise,
    params: [{ ...raise.params![0], min: 60, max: 600 }],
  };

  it('is sent clamped into the range on offer now', () => {
    expect(submissionFor(shorter, { params: { amount: '900' } })?.params?.amount).toBe('600');
    expect(submissionFor(shorter, { params: { amount: '10' } })?.params?.amount).toBe('60');
  });

  it('is titled with that same clamped figure', () => {
    expect(offerHeadline(shorter, { amount: '900' })?.value).toBe('600');
  });

  it('falls back to the default when the field was left empty', () => {
    expect(submissionFor(shorter, { params: { amount: '' } })?.params?.amount).toBe('60');
  });
});
