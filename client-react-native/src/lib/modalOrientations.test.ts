import { MODAL_ORIENTATIONS } from '@/src/lib/modalOrientations';

// iOS draws a Modal in portrait only unless told otherwise, which turned a
// phone held sideways back upright whenever a sheet opened.
it('lets a sheet open either way up', () => {
  expect(MODAL_ORIENTATIONS).toEqual(expect.arrayContaining(['portrait', 'landscape-left', 'landscape-right']));
});
