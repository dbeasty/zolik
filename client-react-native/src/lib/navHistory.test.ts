import { aheadCount, leavingBack, resetNavHistory, routeChanged, takeForward } from './navHistory';

beforeEach(resetNavHistory);

test('forward returns to what back left, newest first', () => {
  leavingBack('/match/2');
  routeChanged();
  leavingBack('/lobby/games');
  routeChanged();
  expect(aheadCount()).toBe(2);
  expect(takeForward()).toBe('/lobby/games');
  routeChanged();
  expect(takeForward()).toBe('/match/2');
  routeChanged();
  expect(takeForward()).toBeNull();
});

test('going anywhere new forgets what was ahead', () => {
  leavingBack('/match/2');
  routeChanged();
  routeChanged(); // a tap on a game
  expect(aheadCount()).toBe(0);
});

test('the route change forward causes keeps the rest', () => {
  leavingBack('/a');
  routeChanged();
  leavingBack('/b');
  routeChanged();
  takeForward();
  routeChanged();
  expect(aheadCount()).toBe(1);
});
