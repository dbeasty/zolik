jest.mock('expo-router', () => ({ router: {}, usePathname: () => '/', useGlobalSearchParams: () => ({}), useSegments: () => [] }));

import { routeSearch } from './windows';

describe('routeSearch', () => {
  it("keeps the rules screen's query, which a game window hands to the main window", () => {
    expect(
      routeSearch({ moduleId: 'canasta', variation: 'canastax', options: '{}' }, ['rules']),
    ).toBe('?moduleId=canasta&variation=canastax&options=%7B%7D');
  });

  it('leaves out the params the path already carries', () => {
    expect(routeSearch({ matchId: 'abc' }, ['match', '[matchId]'])).toBe('');
    expect(routeSearch({ code: 'x', ref: 'y' }, ['r', '[code]'])).toBe('?ref=y');
  });

  it('is empty without params', () => {
    expect(routeSearch({}, ['settings'])).toBe('');
  });
});
