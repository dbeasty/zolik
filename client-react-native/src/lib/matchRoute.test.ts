import { routeForMatch, routeForReplay } from '@/src/lib/matchRoute';

describe('routeForMatch', () => {
  it('sends a dealt table straight to the board, for host and guest alike', () => {
    for (const status of ['active', 'completed', 'suspended', 'abandoned']) {
      expect(routeForMatch(status, true, 'm1')).toBe('/match/m1');
      expect(routeForMatch(status, false, 'm1')).toBe('/match/m1');
    }
  });

  it('sends a lobby host to the table screen and everyone else to the waiting screen', () => {
    expect(routeForMatch('lobby', true, 'm1')).toBe('/lobby/table?matchId=m1');
    expect(routeForMatch('lobby', false, 'm1')).toBe('/lobby/join?matchId=m1');
  });

  it('encodes an id that is not URL-safe, so it cannot break out of the path or query', () => {
    expect(routeForMatch('active', false, 'a/b c')).toBe('/match/a%2Fb%20c');
    expect(routeForMatch('lobby', true, 'x&y=1')).toBe('/lobby/table?matchId=x%26y%3D1');
  });
});

describe('routeForReplay', () => {
  it('is its own route, not the board', () => {
    expect(routeForReplay('m1')).toBe('/replay/m1');
    expect(routeForReplay('a/b')).toBe('/replay/a%2Fb');
  });
});
