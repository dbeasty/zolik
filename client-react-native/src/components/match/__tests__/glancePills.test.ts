import type { ActionOffer } from '@/src/api/matchTypes';
import { glancePills } from '@/src/components/match/OfferBar';

/**
 * The collapsed Controls strip shows a few pills and folds the rest into "+N".
 * Modules list undo offers last, so a plain first-four cut hid "Undo meld" in
 * CanastaX the moment a meld went down.
 */
describe('the collapsed Controls strip', () => {
  const offer = (id: string, undo = false): ActionOffer => ({ id, verb: id, enabled: true, undo });

  // CanastaX right after a meld: four ordinary moves, then the take-back.
  const canastaX = [
    offer('discard'),
    offer('lay_meld'),
    offer('lay_off'),
    offer('swap_wild'),
    offer('undo_lay_meld', true),
  ];

  it('keeps the undo when ordinary moves would fill it', () => {
    const { shown, rest } = glancePills(canastaX, 4);
    expect(shown.map((o) => o.id)).toEqual(['discard', 'lay_meld', 'lay_off', 'undo_lay_meld']);
    expect(rest).toBe(1);
  });

  it('keeps the server order, undo last', () => {
    const { shown } = glancePills([offer('undo_lay_off', true), offer('discard')], 4);
    expect(shown.map((o) => o.id)).toEqual(['undo_lay_off', 'discard']);
  });

  it('is the plain first-max cut when there is no undo', () => {
    const { shown, rest } = glancePills(canastaX.slice(0, 4).concat(offer('extra')), 4);
    expect(shown.map((o) => o.id)).toEqual(['discard', 'lay_meld', 'lay_off', 'swap_wild']);
    expect(rest).toBe(1);
  });

  it('shows everything when it fits', () => {
    expect(glancePills(canastaX, 8)).toEqual({ shown: canastaX, rest: 0 });
  });
});
