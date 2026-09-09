import { storage } from '@/src/context/SessionContext';
import {
  clearPendingInvite,
  consumePendingInvite,
  peekPendingInvite,
  savePendingInvite,
} from '@/src/lib/pendingInvite';

/**
 * The note that carries an invite across sign-in.
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

it('remembers a code across the sign-in detour', async () => {
  await savePendingInvite('ABC123');
  expect(await peekPendingInvite()).toBe('ABC123');
});

it('consumes the note exactly once', async () => {
  // The property that keeps a followed invite from ambushing somebody on
  // their next visit to the main menu, which reads the note on every mount.
  await savePendingInvite('ABC123');
  expect(await consumePendingInvite()).toBe('ABC123');
  expect(await consumePendingInvite()).toBe('');
  expect(await peekPendingInvite()).toBe('');
});

it('has nothing to report when nothing was saved', async () => {
  expect(await peekPendingInvite()).toBe('');
  expect(await consumePendingInvite()).toBe('');
});

it('ignores an empty code rather than storing a note that means nothing', async () => {
  await savePendingInvite('   ');
  expect(await peekPendingInvite()).toBe('');
});

it('trims what it stores', async () => {
  await savePendingInvite('  ABC123 ');
  expect(await peekPendingInvite()).toBe('ABC123');
});

it('forgets a note older than half an hour', async () => {
  await savePendingInvite('ABC123');

  // Not a mocked clock but a rewritten note: the age is a stored timestamp, so
  // ageing the note is the honest way to test the rule that reads it.
  const key = [...mockBacking.keys()][0];
  const note = JSON.parse(mockBacking.get(key)!);
  mockBacking.set(key, JSON.stringify({ ...note, at: Date.now() - 31 * 60 * 1000 }));

  expect(await peekPendingInvite()).toBe('');
  // And it is cleaned up on the way past, not left to be re-checked forever.
  expect(mockBacking.size).toBe(0);
});

it('keeps a note that is merely old-ish', async () => {
  // A sign-in that goes out to an identity provider and back can take minutes.
  // The window has to be comfortably longer than the slowest honest case.
  await savePendingInvite('ABC123');
  const key = [...mockBacking.keys()][0];
  const note = JSON.parse(mockBacking.get(key)!);
  mockBacking.set(key, JSON.stringify({ ...note, at: Date.now() - 10 * 60 * 1000 }));

  expect(await peekPendingInvite()).toBe('ABC123');
});

it('treats an unreadable note as no note', async () => {
  // Storage written by an older build, or truncated. A player lands on the
  // menu instead of the table — a worse experience, not a crash.
  mockBacking.set('zolik_pending_invite', '{not json');
  expect(await peekPendingInvite()).toBe('');

  mockBacking.set('zolik_pending_invite', JSON.stringify({ code: 'ABC123' }));
  expect(await peekPendingInvite()).toBe('');
});

it('survives a storage that refuses to write', async () => {
  // A private window. Losing the note is acceptable; throwing out of the
  // sign-in handler that called this is not.
  (storage.setItem as jest.Mock).mockRejectedValueOnce(new Error('denied'));
  await expect(savePendingInvite('ABC123')).resolves.toBeUndefined();
});

it('survives a storage that refuses to delete', async () => {
  await savePendingInvite('ABC123');
  (storage.deleteItem as jest.Mock).mockRejectedValueOnce(new Error('denied'));
  await expect(clearPendingInvite()).resolves.toBeUndefined();
});
