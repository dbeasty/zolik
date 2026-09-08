import { LOCALES } from '@/src/lib/i18n';
import { detectLocale, localeFromTag } from '@/src/lib/localeDetect';

describe('localeFromTag', () => {
  it('takes a bare language tag', () => {
    expect(localeFromTag('cs')).toBe('cs');
    expect(localeFromTag('mt')).toBe('mt');
  });

  it('drops the region, in either separator', () => {
    // The web says `cs-CZ`, Apple says `cs_CZ`, and both mean Czech.
    expect(localeFromTag('cs-CZ')).toBe('cs');
    expect(localeFromTag('cs_CZ')).toBe('cs');
    expect(localeFromTag('el-CY')).toBe('el');
  });

  it('serves one Portuguese to every region that asks for Portuguese', () => {
    // The pt we ship is European. A Brazilian reader is far better served by
    // it than by English, which is what a strict region match would give them.
    expect(localeFromTag('pt-BR')).toBe('pt');
    expect(localeFromTag('pt-PT')).toBe('pt');
    expect(localeFromTag('de-AT')).toBe('de');
    expect(localeFromTag('nl-BE')).toBe('nl');
    expect(localeFromTag('fr-CA')).toBe('fr');
  });

  it('drops a script subtag', () => {
    expect(localeFromTag('sr-Latn-RS')).toBeUndefined();
    expect(localeFromTag('de-Latn-DE')).toBe('de');
  });

  it('accepts the three-letter spellings some platforms report', () => {
    expect(localeFromTag('deu')).toBe('de');
    expect(localeFromTag('ger')).toBe('de'); // bibliographic
    expect(localeFromTag('gle')).toBe('ga');
    expect(localeFromTag('slo')).toBe('sk'); // not Slovenian — a real trap
    expect(localeFromTag('slv')).toBe('sl');
  });

  it('is case- and whitespace-insensitive', () => {
    expect(localeFromTag('  FR-fr ')).toBe('fr');
    expect(localeFromTag('DE')).toBe('de');
  });

  it('refuses a language we have no bundle for', () => {
    // Not EU-official, so not shipped — and deliberately NOT mapped to a
    // neighbour. Norwegian is not Danish and Bosnian is not Croatian; being
    // wrong about that is worse than being unhelpful.
    expect(localeFromTag('nb')).toBeUndefined();
    expect(localeFromTag('bs')).toBeUndefined();
    expect(localeFromTag('ca')).toBeUndefined();
    expect(localeFromTag('ru')).toBeUndefined();
    expect(localeFromTag('is')).toBeUndefined();
  });

  it('survives junk rather than throwing at launch', () => {
    expect(localeFromTag('')).toBeUndefined();
    expect(localeFromTag(null)).toBeUndefined();
    expect(localeFromTag(undefined)).toBeUndefined();
    expect(localeFromTag('-')).toBeUndefined();
    expect(localeFromTag('@@@')).toBeUndefined();
  });

  it('round-trips every locale the picker offers', () => {
    for (const l of LOCALES) expect(localeFromTag(l.id)).toBe(l.id);
  });
});

describe('detectLocale', () => {
  it('takes the first tag it has a bundle for, not the first tag', () => {
    // The ordered preference list is the player saying "Irish if you have it,
    // otherwise English". Honouring the order is the whole reason we read a
    // list instead of one value.
    expect(detectLocale(['ga-IE', 'en-IE'])).toBe('ga');
    expect(detectLocale(['is-IS', 'da-DK', 'en-GB'])).toBe('da');
  });

  it('skips languages we do not ship rather than stopping at them', () => {
    expect(detectLocale(['ru-RU', 'uk-UA', 'bg-BG'])).toBe('bg');
  });

  it('falls back to English when nothing matches', () => {
    expect(detectLocale(['ja-JP', 'ko-KR'])).toBe('en');
    expect(detectLocale([])).toBe('en');
  });
});
