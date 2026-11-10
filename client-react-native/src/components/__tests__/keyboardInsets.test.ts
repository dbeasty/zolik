import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * iOS leaves a ScrollView where it is when the keyboard opens, so a field low on
 * a page sits behind it: on an iPhone the join code and its button could not be
 * seen while typing. Every scroller that can hold a text field has to say so,
 * and only a real device shows it going wrong.
 */
describe('scrollers holding text fields move out of the keyboard’s way', () => {
  it.each(['src/components/Screen.tsx', 'app/lobby/setup.tsx', 'app/match/[matchId].tsx'])('%s', (file) => {
    const source = readFileSync(join(__dirname, '..', '..', '..', file), 'utf8');
    expect(source).toMatch(/automaticallyAdjustKeyboardInsets/);
  });
});
