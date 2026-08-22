import { BUNDLES, LOCALES, countLabel, getLocale, reasonText, setLocale, t } from '@/src/lib/i18n';

afterEach(() => setLocale('en'));

describe('bundle completeness', () => {
  // The failure this prevents is not a crash — it is a half-translated screen
  // that ships and nobody notices, because the fallback quietly serves
  // English for whichever keys were forgotten. Comparing key sets is the only
  // way to see it before a player does.
  const reference = Object.keys(BUNDLES.en).sort();

  it.each(LOCALES.map((l) => l.id))('%s has exactly the same keys as en', (locale) => {
    const keys = Object.keys(BUNDLES[locale]).sort();
    const missing = reference.filter((k) => !keys.includes(k));
    const extra = keys.filter((k) => !reference.includes(k));
    expect({ missing, extra }).toEqual({ missing: [], extra: [] });
  });

  it.each(LOCALES.map((l) => l.id))('%s has no empty translations', (locale) => {
    const blank = Object.entries(BUNDLES[locale])
      .filter(([, v]) => v.trim() === '')
      .map(([k]) => k);
    expect(blank).toEqual([]);
  });

  it.each(LOCALES.map((l) => l.id))(
    '%s keeps every placeholder its English counterpart uses',
    (locale) => {
      // A translation that drops `{n}` renders "After deals" — grammatical,
      // plausible, and missing the number. Silent, so it has to be checked.
      const placeholders = (s: string) => (s.match(/\{(\w+)\}/g) ?? []).sort();
      const mismatched: string[] = [];
      for (const [key, english] of Object.entries(BUNDLES.en)) {
        const translated = BUNDLES[locale][key];
        if (!translated) continue;
        if (placeholders(english).join() !== placeholders(translated).join()) {
          mismatched.push(`${key}: en${placeholders(english)} vs ${locale}${placeholders(translated)}`);
        }
      }
      expect(mismatched).toEqual([]);
    },
  );

  it('every locale offered in the picker actually has a bundle', () => {
    for (const l of LOCALES) {
      expect(BUNDLES[l.id]).toBeDefined();
      expect(l.label.trim()).not.toBe('');
    }
  });
});

describe('t', () => {
  it('returns the active locale wording', () => {
    setLocale('cs');
    expect(t('rules.variation')).toBe('Varianta');
    setLocale('en');
    expect(t('rules.variation')).toBe('Variation');
  });

  it('interpolates named params', () => {
    expect(t('rules.cards', { n: 13 })).toBe('13 cards');
    setLocale('cs');
    expect(t('rules.cards', { n: 13 })).toBe('13 karet');
  });

  it('leaves an unsupplied placeholder alone rather than printing undefined', () => {
    expect(t('rules.cards')).toBe('{n} cards');
  });

  it('falls back to English for a key the locale lacks', () => {
    // Simulates a partially-translated bundle: the completeness test above
    // stops that shipping, but the fallback must still be correct if it does.
    const saved = BUNDLES.cs['rules.variation'];
    delete BUNDLES.cs['rules.variation'];
    setLocale('cs');
    expect(t('rules.variation')).toBe('Variation');
    BUNDLES.cs['rules.variation'] = saved;
  });

  it('falls back to the caller fallback, then the key, for an unknown key', () => {
    expect(t('no.such.key', undefined, 'Something')).toBe('Something');
    expect(t('no.such.key')).toBe('no.such.key');
  });
});

describe('setLocale', () => {
  it('ignores an unknown locale rather than blanking the UI', () => {
    setLocale('klingon' as never);
    expect(getLocale()).toBe('en');
    expect(t('rules.variation')).toBe('Variation');
  });
});

describe('reasonText', () => {
  it('translates an engine error code', () => {
    expect(reasonText('DISCARD_LOCKED')).toBe('The discard pile is locked for now');
    setLocale('cs');
    expect(reasonText('DISCARD_LOCKED')).toBe('Odhazovací balíček je zatím zamčený');
  });

  it('never leaks a raw code at the player', () => {
    // A server newer than this build can send a code the bundle has not seen.
    expect(reasonText('SOME_FUTURE_CODE', 'Not available')).toBe('Not available');
    expect(reasonText(undefined, 'fine')).toBe('fine');
  });
});

describe('countLabel', () => {
  it('uses the whole phrase per count, in each language', () => {
    expect(countLabel(1, 'sets')).toBe('One set');
    expect(countLabel(2, 'runs')).toBe('Two runs');
    setLocale('cs');
    // Czech inflects the noun by count — "Jedna skupina" vs "Dvě skupiny" —
    // which is exactly what a number-plus-pluralised-noun helper cannot do.
    expect(countLabel(1, 'sets')).toBe('Jedna skupina');
    expect(countLabel(2, 'sets')).toBe('Dvě skupiny');
    expect(countLabel(3, 'runs')).toBe('Tři postupky');
  });

  it('falls back to a generic form for a count it has no phrase for', () => {
    expect(countLabel(7, 'sets')).toBe('7 sets');
    setLocale('cs');
    expect(countLabel(7, 'sets')).toBe('7 skupin');
  });
});
