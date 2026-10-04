import fs from 'fs';
import path from 'path';

import { en } from './locales/en';

// The game MCP server (server/internal/gamemcp) embeds a copy of this bundle
// so it can word the board for an AI client the way the app words it for a
// person. The copy is generated (`cd server && go generate ./internal/gamemcp`)
// and a Go test compares it — but a change to en.ts is made and tested here,
// and three times in a week a new string reached main with the copy stale and
// the Go suite failing. This is the same check where the string is added.
const copyPath = path.join(__dirname, '../../../server/internal/gamemcp/messages_en.json');

const describeIfServer = fs.existsSync(copyPath) ? describe : describe.skip;

describeIfServer('the game MCP server copy of the English bundle', () => {
  const copy = JSON.parse(fs.readFileSync(copyPath, 'utf8')) as Record<string, string>;
  const bundle = en as Record<string, unknown>;

  it('has every key en.ts has, and no other (run: cd server && go generate ./internal/gamemcp)', () => {
    expect(Object.keys(copy).sort()).toEqual(Object.keys(bundle).sort());
  });

  it('words each key as en.ts does (run: cd server && go generate ./internal/gamemcp)', () => {
    const differ = Object.keys(bundle).filter((k) => copy[k] !== bundle[k]);
    expect(differ).toEqual([]);
  });
});
