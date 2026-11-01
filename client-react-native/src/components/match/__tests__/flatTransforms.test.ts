import { readdirSync, readFileSync } from 'fs';
import { join } from 'path';

/**
 * Nothing on the match board is turned in 3D.
 *
 * On iOS a layer drawn with a `perspective` and a `rotateY` blanked the table
 * beside it: every card that landed on the Last Card discard pile flipped in
 * that way, and for the quarter-second of the turn the stock was cut in half
 * and the rest of the pile vanished. Browsers draw the same transform
 * correctly, so nothing the web suite looks at could see it. The turns are
 * drawn flat now (a `scaleX`), and this keeps them that way.
 */
describe('match board transforms', () => {
  const dir = join(__dirname, '..');
  const sources = readdirSync(dir).filter((f) => f.endsWith('.tsx'));

  it.each(sources)('%s uses no 3D transform', (file) => {
    const source = readFileSync(join(dir, file), 'utf8');
    expect(source).not.toMatch(/\{\s*(perspective|rotateX|rotateY)\s*:/);
  });
});
