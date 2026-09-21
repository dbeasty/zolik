import fs from 'fs';

/**
 * The server words OS pushes from `server/internal/notify/texts.json`, which
 * is generated from these bundles by `scripts/gen-notify-texts.js`. This is
 * the lock between the two: reword `notify.push.*` or a game name here and
 * forget to regenerate, and this fails instead of phones showing the old
 * words.
 *
 * Fix with `node scripts/gen-notify-texts.js` from the repository root.
 */
// eslint-disable-next-line @typescript-eslint/no-require-imports
const gen = require('../../../scripts/gen-notify-texts.js') as {
  render: () => string;
  build: () => Record<string, Record<string, string>>;
  outFile: string;
};

describe('push texts for the server', () => {
  it('texts.json matches the locale bundles', () => {
    expect(fs.readFileSync(gen.outFile, 'utf8')).toBe(gen.render());
  });

  it('words the invite in every locale with the placeholders the server fills', () => {
    const texts = gen.build();
    expect(Object.keys(texts)).toHaveLength(24);
    for (const [locale, t] of Object.entries(texts)) {
      expect([locale, t['notify.push.inviteTitle']]).toEqual([locale, expect.stringContaining('{host}')]);
      expect([locale, t['notify.push.inviteBody']]).toEqual([locale, expect.stringContaining('{game}')]);
    }
  });
});
