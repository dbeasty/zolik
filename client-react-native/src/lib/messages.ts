/**
 * Wording for the server's stable error-code vocabulary.
 *
 * The table itself moved into src/lib/i18n.ts when a second locale arrived —
 * an English-only map next to a bundle is just a locale that cannot be
 * translated. This re-export stays so callers keep one obvious import for
 * "turn an engine code into words".
 */

export { reasonText } from '@/src/lib/i18n';
