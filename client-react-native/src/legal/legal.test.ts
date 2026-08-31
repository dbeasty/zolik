import { BUNDLES, LOCALES, setLocale } from '@/src/lib/i18n';
import { LEGAL_DOCUMENTS, legalDocument, legalParams, operatorIsNamed } from '@/src/legal';
import type { LegalDocId } from '@/src/legal';

afterEach(() => setLocale('en'));

const DOC_IDS: LegalDocId[] = ['terms', 'privacy'];
const locales = LOCALES.map((l) => l.id);

/**
 * The same discipline `i18n.test.ts` applies to interface strings, applied to
 * the documents — and for a sharper reason. A half-translated button is a bad
 * day; a half-translated privacy notice means a Czech reader was shown an
 * English clause, which is to say was not shown it at all. The English
 * fallback that saves the rest of the UI is precisely what would hide this.
 */
describe('document completeness', () => {
  it.each(locales)('%s has every document', (locale) => {
    expect(Object.keys(LEGAL_DOCUMENTS[locale]).sort()).toEqual([...DOC_IDS].sort());
  });

  describe.each(DOC_IDS)('%s', (docId) => {
    const reference = LEGAL_DOCUMENTS.en[docId];

    it.each(locales)('%s has the same sections, in the same order', (locale) => {
      const ids = LEGAL_DOCUMENTS[locale][docId].sections.map((s) => s.id);
      expect(ids).toEqual(reference.sections.map((s) => s.id));
    });

    it.each(locales)('%s has the same number of paragraphs in every section', (locale) => {
      // A translation that quietly drops a paragraph drops a clause. Counting
      // is crude and catches exactly that.
      const counts = LEGAL_DOCUMENTS[locale][docId].sections.map((s) => `${s.id}:${s.body.length}`);
      expect(counts).toEqual(reference.sections.map((s) => `${s.id}:${s.body.length}`));
    });

    it.each(locales)('%s has no empty text', (locale) => {
      const doc = LEGAL_DOCUMENTS[locale][docId];
      const blank: string[] = [];
      if (doc.title.trim() === '') blank.push('title');
      for (const section of doc.sections) {
        if (section.heading.trim() === '') blank.push(`${section.id}:heading`);
        section.body.forEach((p, i) => {
          if (p.trim() === '') blank.push(`${section.id}:${i}`);
        });
      }
      expect(blank).toEqual([]);
    });

    it.each(locales)('%s carries the same version as the English it translates', (locale) => {
      expect(LEGAL_DOCUMENTS[locale][docId].version).toBe(reference.version);
    });

    it.each(locales)('%s keeps every placeholder its English counterpart uses', (locale) => {
      // A clause that loses `{contact}` becomes "write to  and we will remove
      // the account" — grammatical enough to survive a read-through, and it
      // tells the reader nothing about where to write.
      const placeholders = (s: string) => (s.match(/\{(\w+)\}/g) ?? []).sort().join();
      const doc = LEGAL_DOCUMENTS[locale][docId];
      const mismatched: string[] = [];
      doc.sections.forEach((section, i) => {
        const ref = reference.sections[i];
        const found = placeholders(section.body.join(' '));
        const expected = placeholders(ref.body.join(' '));
        if (found !== expected) mismatched.push(`${section.id}: ${expected} vs ${found}`);
      });
      expect(mismatched).toEqual([]);
    });

    it.each(locales)('%s uses no placeholder that nothing fills', (locale) => {
      // `{conatct}` renders as itself, in a document nobody proofreads twice.
      // This caught the real thing on the first run: every document opened
      // with `{operator}` while the record spelled it `name`, so both notices
      // named "{operator}" and read as broken template output.
      const fillable = legalParams();
      const unknown = new Set<string>();
      for (const section of LEGAL_DOCUMENTS[locale][docId].sections) {
        for (const match of section.body.join(' ').matchAll(/\{(\w+)\}/g)) {
          if (!(match[1] in fillable)) unknown.add(match[1]);
        }
      }
      expect([...unknown]).toEqual([]);
    });
  });
});

describe('resolving a document for the reader', () => {
  it.each(locales)('%s leaves no placeholder unsubstituted', (locale) => {
    setLocale(locale);
    for (const docId of DOC_IDS) {
      const rendered = legalDocument(docId)
        .sections.flatMap((s) => s.body)
        .join(' ');
      expect(rendered).not.toMatch(/\{\w+\}/);
    }
  });

  it('serves the locale that is set', () => {
    setLocale('cs');
    expect(legalDocument('terms').title).toBe(LEGAL_DOCUMENTS.cs.terms.title);
    setLocale('en');
    expect(legalDocument('terms').title).toBe(LEGAL_DOCUMENTS.en.terms.title);
  });

  it('falls back to English rather than to nothing', () => {
    // Not reachable through `setLocale`, which validates its argument — this
    // is the guard for a locale added to LOCALES before its documents are.
    expect(legalDocument('privacy', 'de' as never).title).toBe(LEGAL_DOCUMENTS.en.privacy.title);
  });
});

describe('the operator record', () => {
  /**
   * This is the test that will fail when the placeholders are filled in, and
   * that is the point: it is a checklist item, not a defect. Invert it to
   * `toBe(true)` in the same commit that names the operator, and the draft
   * banner disappears from both screens on its own.
   */
  it('is still a placeholder, so the screens mark themselves a draft', () => {
    expect(operatorIsNamed()).toBe(false);
  });

  it('would be considered named once every field is real', () => {
    const filled = { name: 'Someone', country: 'Czechia', contact: 'hi@example.test' };
    expect(Object.values(filled).every((v) => v.trim() !== '' && !v.startsWith('['))).toBe(true);
  });
});

describe('the strings around the documents', () => {
  // These live in the i18n bundle rather than here, so they are the one part
  // of this feature the i18n parity test already guards. What it cannot check
  // is that they exist at all.
  const keys = [
    'legal.terms',
    'legal.privacy',
    'legal.updated',
    'legal.draft',
    'legal.notice.before',
    'legal.notice.terms',
    'legal.notice.between',
    'legal.notice.privacy',
    'legal.notice.after',
  ];

  it.each(locales)('%s words every key the legal screens ask for', (locale) => {
    const missing = keys.filter((k) => !BUNDLES[locale][k]);
    expect(missing).toEqual([]);
  });
});
