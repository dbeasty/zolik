import type { ActionOffer } from '@/src/api/matchTypes';
import { dropSpotsFor, refusalAt, spotAt, takeableSpots } from '@/src/lib/drops';
import { BUNDLES, reasonText, setLocale, t } from '@/src/lib/i18n';

afterEach(() => setLocale('en'));

/**
 * A refused move explains itself, wherever it was refused.
 *
 * The claim under test is not that any particular sentence is right — that is
 * the bundle's business, and serverKeys.test.ts checks it exists. It is that
 * nothing is dropped on the floor: a drop the engine forbids, a drop the
 * current selection does not fit, and a code from a server newer than this
 * build all arrive somewhere a player can read.
 */

const layOff: ActionOffer = {
  id: 'lay_off:meld_0',
  verb: 'lay_off',
  enabled: true,
  source: { zone: 'hand', minCards: 1, cards: ['7H'] },
  target: { zone: 'meld', meldId: 'meld_0' },
};

const discard: ActionOffer = {
  id: 'discard',
  verb: 'discard',
  enabled: false,
  whyNot: 'DISCARD_CARD_NOT_MELDED',
  ruleIds: ['zolik.rules.pickup.obligation'],
  remedy: { labelKey: 'zolik.remedy.meldThePickup', params: { card: '7H' } },
  remedyOfferId: 'undo:draw_discard',
  source: { zone: 'hand', minCards: 1 },
  target: { zone: 'discard_pile', zoneId: 'discard' },
};

describe('a refused drop', () => {
  it('carries the engine reason, its rules and its remedy', () => {
    const spots = dropSpotsFor([discard], ['KD']);
    const refusal = refusalAt(spots, 'zone-discard');

    expect(refusal?.code).toBe('DISCARD_CARD_NOT_MELDED');
    expect(refusal?.ruleIds).toEqual(['zolik.rules.pickup.obligation']);
    expect(refusal?.remedyOfferId).toBe('undo:draw_discard');
  });

  it('is never mistaken for a place the cards may go', () => {
    const spots = dropSpotsFor([discard], ['KD']);
    // Both halves matter: nothing lights up, and nothing sends.
    expect(takeableSpots(spots)).toEqual([]);
    expect(spotAt(spots, 'zone-discard')).toBeUndefined();
  });

  it("explains a selection the offer will not take, not just one the engine refused", () => {
    const spots = dropSpotsFor([layOff], ['2C']);
    expect(refusalAt(spots, 'group-meld_0')?.labelKey).toBe('sel.notThese');
    expect(takeableSpots(spots)).toEqual([]);
  });

  it('still lets a legal drop through untouched', () => {
    const spots = dropSpotsFor([layOff], ['7H']);
    expect(refusalAt(spots, 'group-meld_0')).toBeUndefined();
    expect(spotAt(spots, 'group-meld_0')?.ready).toBe(true);
  });
});

describe('wording a refusal', () => {
  it('words the reason a discard-pile pickup blocks a discard', () => {
    expect(reasonText('DISCARD_CARD_NOT_MELDED')).toBe('The card you picked up must go into your meld');
  });

  it('names the card in the remedy rather than saying "the card you picked up"', () => {
    expect(t('zolik.remedy.meldThePickup', { card: '7♥' })).toContain('7♥');
  });

  it('states the rule behind it in both locales', () => {
    for (const locale of ['en', 'cs'] as const) {
      expect(BUNDLES[locale]['zolik.rules.pickup.obligation']).toBeTruthy();
    }
  });

  it('falls back rather than blanking on a code this build has never seen', () => {
    // A server newer than the app. Ugly, and legible, which is the trade.
    expect(reasonText('SOME_FUTURE_CODE', 'SOME_FUTURE_CODE')).toBe('SOME_FUTURE_CODE');
  });
});
