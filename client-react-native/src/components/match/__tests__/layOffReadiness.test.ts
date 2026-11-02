import type { ActionOffer } from '@/src/api/matchTypes';
import { leadOffer, refusalFor, targetUnready } from '@/src/components/match/OfferBar';

/**
 * A lay-off waits for the cards. It used to light up the moment anything in
 * hand could go on any meld, so a press with nothing picked either sent a card
 * the player never chose or fell through to "pick on the board".
 */
describe('a lay-off control', () => {
  const onto = (meldId: string, cards: string[]): ActionOffer => ({
    id: `lay_off:${meldId}`,
    verb: 'lay_off',
    enabled: true,
    source: { zone: 'hand', minCards: 1, maxCards: 5, cards },
    target: { zone: 'meld', meldId },
  });

  it('is not ready with nothing selected, even when only one card fits', () => {
    const offer = onto('m1', ['7H']);
    expect(leadOffer([offer], [])).toBeUndefined();
    expect(refusalFor(offer, [], undefined)).toMatchObject({ labelKey: 'sel.needMore', params: { n: 1 } });
  });

  it('is ready once the selection fits', () => {
    const offer = onto('m1', ['7H']);
    expect(leadOffer([offer], ['7H'])?.id).toBe(offer.id);
  });

  describe('folded over several melds', () => {
    const group = [onto('m1', ['7H']), onto('m2', ['KS'])];

    it('asks for a card while nothing is selected', () => {
      expect(targetUnready(group, [])).toEqual({ ok: false, labelKey: 'sel.needMore', params: { n: 1 } });
    });

    it('stays off for cards no meld takes', () => {
      expect(targetUnready(group, ['2C'])).toEqual({ ok: false, labelKey: 'sel.notThese' });
    });

    it('lights for cards that fit one of them', () => {
      expect(targetUnready(group, ['KS'])).toBeUndefined();
    });

    it('once a meld is aimed at, judges the selection against that meld only', () => {
      expect(targetUnready(group, ['KS'], 'm1')).toEqual({ ok: false, labelKey: 'sel.notThese' });
      expect(targetUnready(group, ['7H'], 'm1')).toBeUndefined();
    });
  });
});

/**
 * Poaching a wild is the same shape — one natural from hand onto a meld that
 * holds a wild it stands for — and it said "more than one place this could
 * go" for a 10♥ no meld would swap, because each poach lists exactly one
 * card and so read as a bare tap. These are the offers a real CanastaX table
 * sent: the K♣ swaps into one run, the 9♥ into another, nothing else does.
 */
describe('a poach control', () => {
  const poach = (meldId: string, card: string): ActionOffer => ({
    id: `poach:${meldId}`,
    verb: 'poach',
    enabled: true,
    labelKey: 'verb.poach',
    source: { zone: 'hand', minCards: 1, maxCards: 1, cards: [card] },
    target: { zone: 'meld', meldId },
  });
  const group = [poach('t1-seq3', 'KC'), poach('t1-seq4', '9H')];

  it('waits for a card while nothing is selected', () => {
    expect(targetUnready(group, [])).toEqual({ ok: false, labelKey: 'sel.needMore', params: { n: 1 } });
    expect(leadOffer([poach('t1-seq3', 'KC')], [])).toBeUndefined();
  });

  it('stays off for a card that stands in for no wild', () => {
    expect(targetUnready(group, ['TH'])).toEqual({ ok: false, labelKey: 'sel.notThese' });
  });

  it('lights for a card that does', () => {
    expect(targetUnready(group, ['9H'])).toBeUndefined();
    expect(targetUnready(group, ['KC'])).toBeUndefined();
  });
});
