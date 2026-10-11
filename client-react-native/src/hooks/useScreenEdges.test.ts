import { screenEdges } from './useScreenEdges';

describe('screenEdges', () => {
  it('leaves the top to the stack header on a screen that has one', () => {
    for (const path of ['/', '/lobby/games', '/match/abc', '/replay/abc', '/rules']) {
      expect(screenEdges(path)).toEqual(['left', 'right']);
    }
  });
  it('keeps the top inset on a route drawn without a header', () => {
    expect(screenEdges('/auth/callback')).toEqual(['top', 'left', 'right']);
  });
});
