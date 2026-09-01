/**
 * The shape of a legal document.
 *
 * Its own file rather than `index.ts` so a document module can import the type
 * without importing the index that imports it back.
 */

/** Which document. Also the route segment and the i18n key suffix. */
export type LegalDocId = 'terms' | 'privacy';

export type LegalSection = {
  /**
   * A stable address for one section, in the same spirit as a rule id: the
   * heading may be reworded and the order may change, but a section keeps its
   * id forever. That is what lets the locale bundles be checked against each
   * other by id rather than by position, and what makes "which clause was
   * agreed to" answerable later.
   */
  id: string;
  heading: string;
  /** Paragraphs, rendered in order. Each may carry `{placeholder}` params. */
  body: string[];
};

export type LegalDocument = {
  id: LegalDocId;
  title: string;
  /**
   * The date this wording was settled, `YYYY-MM-DD`. Shown to the reader, and
   * asserted identical across locales — a Czech document dated earlier than
   * the English one it translates is a translation nobody redid.
   */
  version: string;
  sections: LegalSection[];
};
