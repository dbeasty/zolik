import type { Zone } from '@/src/api/matchTypes';
import { sayFor, sfxBetween } from '@/src/sound/cues';
import { parseSoundPrefs } from '@/src/sound/prefs';

const cards = (n: number) => Array.from({ length: n }, (_, i) => ({ id: `c${i}`, card: 'AS' }));
const pile = (id: string, n: number): Zone => ({ id, kind: 'pile', count: n, cards: cards(n) as Zone['cards'] });
const stock = (n: number): Zone => ({ id: 'stock', kind: 'stack', count: n });
const hand = (n: number): Zone => ({ id: 'hand', kind: 'hand', count: n, cards: cards(n) as Zone['cards'] });

describe('sfxBetween', () => {
  it('is silent on the first board', () => {
    expect(sfxBetween(null, { zones: [pile('discard', 3)] })).toBeNull();
  });
  it('plays the card sound when a card lands on a pile', () => {
    expect(sfxBetween({ zones: [pile('discard', 3), hand(5)] }, { zones: [pile('discard', 4), hand(4)] })).toBe('card');
  });
  it('plays the take sound when a face-up pile goes to a hand', () => {
    expect(sfxBetween({ zones: [pile('discard', 6), hand(5)] }, { zones: [pile('discard', 0), hand(11)] })).toBe('take');
  });
  it('treats a draw from the hidden stock as a plain card sound', () => {
    expect(sfxBetween({ zones: [stock(40), hand(5)] }, { zones: [stock(39), hand(6)] })).toBe('card');
  });
  it('is silent for a deal, however many cards move', () => {
    expect(sfxBetween({ zones: [stock(52), hand(0)] }, { zones: [stock(41), hand(11)] })).toBeNull();
  });
  it('does not call a reshuffle of the discards a take', () => {
    expect(sfxBetween({ zones: [pile('discard', 5), stock(0)] }, { zones: [pile('discard', 0), stock(5)] })).toBe('card');
  });
});

describe('sayFor', () => {
  it('calls a draw penalty with its count', () => {
    expect(sayFor({ labelKey: 'lastcard.move.playedDraw', params: { n: 2 } })).toEqual({ key: 'sound.say.take', params: { n: 2 } });
  });
  it('calls skip, reverse and last card', () => {
    expect(sayFor({ labelKey: 'lastcard.move.playedSkip' })?.key).toBe('sound.say.skip');
    expect(sayFor({ labelKey: 'lastcard.move.playedReverseClockwise' })?.key).toBe('sound.say.reverse');
    expect(sayFor({ labelKey: 'lastcard.move.called' })?.key).toBe('sound.say.lastCard');
  });
  it('says nothing for an ordinary play', () => {
    expect(sayFor({ labelKey: 'lastcard.move.played' })).toBeNull();
  });
});

describe('parseSoundPrefs', () => {
  it('defaults to effects on, voice off', () => {
    expect(parseSoundPrefs(null)).toEqual({ enabled: true, voice: false });
    expect(parseSoundPrefs('not json')).toEqual({ enabled: true, voice: false });
  });
  it('keeps valid fields and defaults the rest', () => {
    expect(parseSoundPrefs('{"enabled":false,"voice":"yes"}')).toEqual({ enabled: false, voice: false });
  });
});
