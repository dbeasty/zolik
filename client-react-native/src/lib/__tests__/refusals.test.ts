import type { ActionOffer } from '@/src/api/matchTypes';
import { dropSpotsFor, refusalAt, spotAt, takeableSpots } from '@/src/lib/drops';
import { BUNDLES, LOCALES, reasonText, setLocale, t } from '@/src/lib/i18n';
import serverKeys from '@/src/lib/serverKeys.json';

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

/**
 * The sentence a refused player actually reads.
 *
 * The tests above prove a refusal is carried somewhere legible; these prove
 * what it says once it gets there. They exist because the complaint that
 * started this was not "nothing is shown" — it was that what is shown says
 * nothing: `err.WRONG_PHASE` read "Not available right now", and in Czech
 * literally "not now", under every greyed-out control in five games.
 */
describe('what a refusal says', () => {
  it('no longer answers "why not?" with "not now"', () => {
    // The reason narrows it to a moment in the turn; the remedy says which
    // moment and what to do about it. Neither half alone is the fix.
    expect(reasonText('WRONG_PHASE')).toBe('Not at this point in the turn');
    setLocale('cs');
    expect(reasonText('WRONG_PHASE')).not.toBe('Teď to nejde');
  });

  it('names the move to make instead, per game and per phase', () => {
    expect(t('canasta.remedy.drawOrTakePile')).toMatch(/draw|take/i);
    expect(t('canasta.remedy.meldOrDiscard')).toMatch(/discard/i);
    expect(t('ginrummy.remedy.drawFirst')).toMatch(/draw/i);
    expect(t('ginrummy.remedy.discardToEndTurn')).toMatch(/discard/i);
    expect(t('blackjack.remedy.putAStakeUp', { n: 10 })).toMatch(/stake/i);
    expect(t('rummytiles.remedy.playFromHandOrDraw')).toMatch(/play|draw/i);
  });

  it('puts the live figure in the sentence rather than the rule-book number', () => {
    expect(t('holdem.remedy.callOrFold', { n: 40 })).toContain('40');
    expect(t('holdem.remedy.raiseAtLeast', { n: 180 })).toContain('180');
    expect(t('canasta.remedy.needMorePoints', { n: 20 })).toContain('20');
    expect(t('prsi.remedy.answerSevenOrTake', { n: 4 })).toContain('4');
  });

  it('has somewhere to put the tokens the server sends as names', () => {
    // The server sends `suit.H` and a card code; labels.ts turns both into
    // words before they reach the template.
    const sentence = t('prsi.remedy.matchOrDraw', { suit: 'Hearts', card: '7♠' });
    expect(sentence).toContain('Hearts');
    expect(sentence).toContain('7♠');
  });
});

/**
 * Every remedy the server can send reads as a finished sentence in every
 * language — no placeholder left standing where a number should be.
 *
 * `serverKeys.test.ts` checks the other direction: that a template never asks
 * for a parameter the server does not send. This checks that substituting the
 * parameters it does send leaves nothing behind, which is what catches `{nn}`
 * where `{n}` was meant — a typo no parity check sees, because it is a
 * placeholder in every locale equally.
 */
describe('every remedy renders', () => {
  const remedies = (serverKeys.sentenceKeys as string[]).filter((k) => k.includes('.remedy.'));
  const params = serverKeys.paramsByKey as Record<string, string[]>;

  it('has remedies to check at all', () => {
    expect(remedies.length).toBeGreaterThan(20);
  });

  it.each(LOCALES.map((l) => l.id))('%s leaves no placeholder unfilled', (locale) => {
    setLocale(locale);
    const unfilled: string[] = [];
    for (const key of remedies) {
      const sent = Object.fromEntries((params[key] ?? []).map((name) => [name, '·']));
      const rendered = t(key, sent);
      if (/\{\w+\}/.test(rendered)) unfilled.push(`${key}: ${rendered}`);
    }
    expect(unfilled).toEqual([]);
  });
});
