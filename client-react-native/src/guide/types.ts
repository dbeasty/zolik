/**
 * The guided lessons the server ships (`module.Lesson`), as the client reads
 * them. Keys and params only: every word a step says lives in the locale
 * bundles, exactly like a rule or a label.
 */

/** The parts of the match screen a step can point at. */
export type GuideAnchor =
  | 'hand'
  | 'drawPile'
  | 'discard'
  | 'table'
  | 'actionBar'
  | 'status'
  | 'seats'
  | 'score'
  | 'hint'
  | 'log'
  | 'rules';

export type GuideStep = {
  id: string;
  /** `say` is read and dismissed; `act` waits for the player's own move. */
  kind: 'say' | 'act';
  textKey: string;
  params?: Record<string, unknown>;
  anchors?: GuideAnchor[];
  /** Written rules this step is about, for "read the rule". */
  ruleIds?: string[];
  verbs?: string[];
  count?: number;
  strict?: boolean;
  wrongKey?: string;
};

export type GuideLesson = {
  id: string;
  titleKey: string;
  summaryKey?: string;
  seed: number;
  bots: number;
  steps: GuideStep[];
};
