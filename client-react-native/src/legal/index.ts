/**
 * The Terms of Use and the Privacy Notice, resolved for the reader's locale.
 *
 * Why these live here rather than in `src/lib/i18n.ts`: that bundle is a flat
 * map of short interface strings, and forty paragraphs of prose in it would
 * drown the keys it exists to serve. What this module keeps from i18n is the
 * discipline — a reference locale, a parity test, and `{placeholder}`
 * interpolation with the same syntax — so the two never diverge in style.
 *
 * Only the short strings around the documents (the link labels, the notice on
 * the sign-in screens) are in `i18n.ts`, under `legal.*`.
 *
 * Nothing here touches the server. These are client-authored words, not server
 * message keys, so `serverKeys.json` is unaffected and needs no regeneration.
 */

import { OPERATOR_CONTACT, OPERATOR_COUNTRY, OPERATOR_NAME, SOURCE_URL } from '@/src/config';
import { getLocale, type Locale } from '@/src/lib/i18n';

import { privacyCs } from './privacy.cs';
import { privacyEn } from './privacy.en';
import { termsCs } from './terms.cs';
import { termsEn } from './terms.en';
import type { LegalDocId, LegalDocument, LegalSection } from './types';

export type { LegalDocId, LegalDocument, LegalSection } from './types';

/**
 * Who is behind the game, in one place, because both documents name them and
 * a disclaimer with two different answers is worse than one with none.
 *
 * Supplied at build time — `ZOLIK_OPERATOR`, `ZOLIK_OPERATOR_COUNTRY` and
 * `ZOLIK_OPERATOR_CONTACT` in `scripts/deploy.sh`, reaching the bundle as
 * `EXPO_PUBLIC_*` (see `src/config.ts`). A deployment names its own operator;
 * a fork that shipped with ours baked in would be making a claim about us.
 *
 * Anything not supplied falls back to a placeholder written in a shape no real
 * value could take, so `operatorIsNamed` can tell "nobody filled this in" from
 * "this is the answer" without a second flag to forget to flip. A local build
 * therefore shows a draft banner, which is the correct thing for a local build
 * to show.
 */
export const OPERATOR: { name: string; country: string; contact: string } = {
  /** Legal name, or "a private individual operating play.limidus.com". */
  name: OPERATOR_NAME || '[OPERATOR NAME]',
  /** The jurisdiction whose law governs the terms. */
  country: OPERATOR_COUNTRY || '[COUNTRY]',
  /** A monitored address. The privacy notice promises a reply within a month. */
  contact: OPERATOR_CONTACT || '[CONTACT EMAIL]',
};

/**
 * Whether `OPERATOR` has been filled in.
 *
 * The documents are complete and the wiring is finished; what is missing is
 * four facts only the operator knows. Rather than block the feature on them,
 * the screens render and mark themselves a draft while this is false. A draft
 * banner is the honest failure mode: a reader is told not to rely on it, which
 * a document silently naming "[OPERATOR NAME]" would not manage.
 */
export function operatorIsNamed(): boolean {
  return Object.values(OPERATOR).every((v) => v.trim() !== '' && !v.startsWith('['));
}

const DOCUMENTS: Record<Locale, Record<LegalDocId, LegalDocument>> = {
  en: { terms: termsEn, privacy: privacyEn },
  cs: { terms: termsCs, privacy: privacyCs },
};

/** Exported for the parity test, which has to see every bundle at once. */
export const LEGAL_DOCUMENTS = DOCUMENTS;

/**
 * One document, in the reader's locale, with `{operator}`, `{country}`,
 * `{contact}` and `{source}` substituted.
 *
 * Falls back to English for a locale with no document, exactly as `t` does.
 * The parity test means that fallback should never fire; it is here because
 * the alternative when it does is a blank screen where the terms should be.
 */
export function legalDocument(id: LegalDocId, locale: Locale = getLocale()): LegalDocument {
  const doc = DOCUMENTS[locale]?.[id] ?? DOCUMENTS.en[id];
  return {
    ...doc,
    sections: doc.sections.map(fillSection),
  };
}

/**
 * What the documents may write in braces, and what each resolves to.
 *
 * Not `OPERATOR` itself: the record is keyed for the reader of the code
 * (`name`), the placeholders are named for the reader of the document
 * (`{operator}`), and a document reading "{name} operates play.limidus.com"
 * would be worse prose to save one indirection. Exported so the parity test
 * can reject a placeholder nothing here fills — which is how the mismatch
 * between the two spellings was caught in the first place.
 */
export function legalParams(): Record<string, string> {
  return {
    operator: OPERATOR.name,
    country: OPERATOR.country,
    contact: OPERATOR.contact,
    // Not part of `OPERATOR`: the other three are facts about who is running
    // the game and are missing until someone supplies them, while this one is
    // a fact about the code and is always known. It is a placeholder rather
    // than prose in the document because a fork's terms must offer the fork's
    // source, and the document is the same file in every deployment.
    source: SOURCE_URL,
  };
}

function fillSection(section: LegalSection): LegalSection {
  return { ...section, body: section.body.map(fill) };
}

function fill(text: string): string {
  const params = legalParams();
  return text.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? params[name] : whole,
  );
}
