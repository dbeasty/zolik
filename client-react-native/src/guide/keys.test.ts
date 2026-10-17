import lessonKeys from '@/src/guide/lessonKeys.json';
import { BUNDLES, LOCALES } from '@/src/lib/i18n';
import { GUIDE_TOPICS } from '@/src/lib/locales/guide';

/**
 * The client half of the guide's two-sided lock.
 *
 * `lessonKeys.json` is generated from the server's lessons
 * (`go run ./cmd/dump-lessons`) and a Go test keeps it current. Here: every
 * key a lesson can show has wording in every language, and every guide topic
 * is complete — a lesson card that fell back to English, or to a bare key, is
 * a guide the player cannot follow.
 */
describe('guide words', () => {
  it.each(LOCALES.map((l) => l.id))('%s words every key a lesson can show', (locale) => {
    const missing = (lessonKeys as string[]).filter((k) => !BUNDLES[locale][k]);
    expect(missing).toEqual([]);
  });

  it.each(Object.keys(GUIDE_TOPICS))('topic %s is written in every language, with the same keys', (topic) => {
    const doc = GUIDE_TOPICS[topic];
    const reference = Object.keys(doc.en).sort();
    for (const l of LOCALES) {
      expect(Object.keys(doc[l.id] ?? {}).sort()).toEqual(reference);
    }
  });

  it('words what the guide says by itself', () => {
    // These are chosen by the client, not named by a lesson: the held-back
    // move, the progress line, the exit questions.
    for (const key of [
      'guide.common.title',
      'guide.common.progress',
      'guide.common.notYet',
      'guide.common.readFirst',
      'guide.exit.title',
      'guide.exit.playForReal',
    ]) {
      for (const l of LOCALES) expect(BUNDLES[l.id][key]).toBeTruthy();
    }
  });
});
