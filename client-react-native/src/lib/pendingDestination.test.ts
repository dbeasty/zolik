import { storage } from '@/src/context/SessionContext';
import {
  clearPendingDestination,
  consumePendingDestination,
  peekPendingDestination,
  savePendingDestination,
} from '@/src/lib/pendingDestination';

/**
 * The note that carries a destination across sign-in.
 *
 * Two properties are worth pinning, and they pull against each other: it must
 * survive long enough to cross a sign-in that may include an OAuth round trip
 * out of the app entirely, and it must not survive so well that opening the
 * app the next morning drags somebody into a table that finished last night.
 */

// A stand-in for the device's store. The real one is SecureStore on native and
// localStorage on web; neither exists under jest, and neither is what these
// tests are about.
const mockBacking = new Map<string, string>();

jest.mock('@/src/context/SessionContext', () => ({
  storage: {
    getItem: jest.fn(async (k: string) => mockBacking.get(k) ?? null),
    setItem: jest.fn(async (k: string, v: string) => {
      mockBacking.set(k, v);
    }),
    deleteItem: jest.fn(async (k: string) => {
      mockBacking.delete(k);
    }),
  },
}));

beforeEach(() => {
  mockBacking.clear();
  jest.clearAllMocks();
  jest.useRealTimers();
});

it('remembers a route across the sign-in detour', async () => {
  await savePendingDestination('/join/ABC123');
  expect(await peekPendingDestination()).toBe('/join/ABC123');
});

// The widening this module exists for: a match link is the other thing people
// share, and it used to have nowhere to be remembered.
it('remembers a match route as readily as an invite one', async () => {
  await savePendingDestination('/match/6aa0c6e184707c14c47d4c31');
  expect(await consumePendingDestination()).toBe('/match/6aa0c6e184707c14c47d4c31');
});

it('consumes the note exactly once', async () => {
  // The property that keeps a followed link from ambushing somebody on their
  // next visit to the main menu, which reads the note on every mount.
  await savePendingDestination('/join/ABC123');
  expect(await consumePendingDestination()).toBe('/join/ABC123');
  expect(await consumePendingDestination()).toBe('');
  expect(await peekPendingDestination()).toBe('');
});

it('has nothing to report when nothing was saved', async () => {
  expect(await peekPendingDestination()).toBe('');
  expect(await consumePendingDestination()).toBe('');
});

it('ignores an empty route rather than storing a note that means nothing', async () => {
  await savePendingDestination('   ');
  expect(await peekPendingDestination()).toBe('');
});

it('trims what it stores', async () => {
  await savePendingDestination('  /join/ABC123 ');
  expect(await peekPendingDestination()).toBe('/join/ABC123');
});

// The note is replayed into router.replace by a screen that cannot judge it,
// and on web it lives in localStorage, which is not a trusted input.
it('refuses a destination that is not an in-app route', async () => {
  for (const hostile of ['https://example.com/phish', '//example.com', 'javascript:alert(1)', 'lobby/games']) {
    await savePendingDestination(hostile);
    expect(await peekPendingDestination()).toBe('');
  }
});

// A rule tightened in a later build has to apply to notes an earlier one
// already wrote, so the check is on the way out as well as the way in.
it('refuses a stored note that is not an in-app route', async () => {
  mockBacking.set(
    'zolik_pending_destination',
    JSON.stringify({ path: 'https://example.com/phish', at: Date.now() }),
  );
  expect(await peekPendingDestination()).toBe('');
});

it('forgets a note older than half an hour', async () => {
  await savePendingDestination('/join/ABC123');

  // Not a mocked clock but a rewritten note: the age is a stored timestamp, so
  // ageing the note is the honest way to test the rule that reads it.
  const key = [...mockBacking.keys()][0];
  const note = JSON.parse(mockBacking.get(key)!);
  mockBacking.set(key, JSON.stringify({ ...note, at: Date.now() - 31 * 60 * 1000 }));

  expect(await peekPendingDestination()).toBe('');
  // And it is cleaned up on the way past, not left to be re-checked forever.
  expect(mockBacking.size).toBe(0);
});

it('keeps a note that is merely old-ish', async () => {
  // A sign-in that goes out to an identity provider and back can take minutes.
  // The window has to be comfortably longer than the slowest honest case.
  await savePendingDestination('/join/ABC123');
  const key = [...mockBacking.keys()][0];
  const note = JSON.parse(mockBacking.get(key)!);
  mockBacking.set(key, JSON.stringify({ ...note, at: Date.now() - 10 * 60 * 1000 }));

  expect(await peekPendingDestination()).toBe('/join/ABC123');
});

it('treats an unreadable note as no note', async () => {
  // Storage written by an older build, or truncated. A player lands on the
  // menu instead of the table — a worse experience, not a crash.
  mockBacking.set('zolik_pending_destination', '{not json');
  expect(await peekPendingDestination()).toBe('');

  mockBacking.set('zolik_pending_destination', JSON.stringify({ path: '/join/ABC123' }));
  expect(await peekPendingDestination()).toBe('');
});

it('survives a storage that refuses to write', async () => {
  // A private window. Losing the note is acceptable; throwing out of the
  // sign-in handler that called this is not.
  (storage.setItem as jest.Mock).mockRejectedValueOnce(new Error('denied'));
  await expect(savePendingDestination('/join/ABC123')).resolves.toBeUndefined();
});

it('survives a storage that refuses to delete', async () => {
  await savePendingDestination('/join/ABC123');
  (storage.deleteItem as jest.Mock).mockRejectedValueOnce(new Error('denied'));
  await expect(clearPendingDestination()).resolves.toBeUndefined();
});
