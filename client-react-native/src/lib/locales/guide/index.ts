/**
 * The words of the guided lessons and the app tour.
 *
 * Kept apart from the locale files for one reason: a game's lesson is written
 * once, by whoever adds the game's guide, and arrives with all twenty-four
 * languages in a single file of its own. One file per topic, each
 * `{ locale: { key: text } }`, means adding a game's guide never touches any
 * other game's words, and a merge of two guides never conflicts.
 *
 * Merged into every locale's bundle in `i18n.ts`, so everything that checks
 * the bundles — same keys everywhere, no blanks, placeholders kept — checks
 * these too. A topic is a JSON file here, listed once below. Add a game's
 * guide by filling in its file (English first; `scripts/guide-merge.js` folds
 * in the translations).
 */

import blackjack from './blackjack.json';
import canasta from './canasta.json';
import common from './common.json';
import ferbl from './ferbl.json';
import ginrummy from './ginrummy.json';
import holdem from './holdem.json';
import klondike from './klondike.json';
import lastcard from './lastcard.json';
import lora from './lora.json';
import marias from './marias.json';
import okobere from './okobere.json';
import prsi from './prsi.json';
import rummytiles from './rummytiles.json';
import sedma from './sedma.json';
import sietemedio from './sietemedio.json';
import snaps from './snaps.json';
import tour from './tour.json';
import zolik from './zolik.json';

type Topic = Record<string, Record<string, string>>;

/** One topic per game (named for its module id), plus what the guide says itself. */
export const GUIDE_TOPICS: Record<string, Topic> = {
  common,
  tour,
  blackjack,
  canasta,
  ferbl,
  ginrummy,
  holdem,
  klondike,
  lastcard,
  lora,
  marias,
  okobere,
  prsi,
  rummytiles,
  sedma,
  sietemedio,
  snaps,
  zolik,
};

/** Every guide string for one language, from every topic. */
export function guideStrings(locale: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const topic of Object.values(GUIDE_TOPICS)) Object.assign(out, topic[locale] ?? {});
  return out;
}
