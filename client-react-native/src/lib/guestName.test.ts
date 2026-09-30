import { avatarFor } from '@/src/components/avatars/catalogue';
import { GUEST_NAME_COUNT, guestNameFor } from '@/src/lib/guestName';

/**
 * The defect these replace is not subtle — every guest was called "Player" —
 * so what is worth asserting is the two properties that stop it coming back:
 * different devices get different names, and one device gets the same name
 * twice.
 */

const GUEST_ID = '0123456789abcdef0123456789abcdef';

describe('guestNameFor', () => {
  it('gives one device the same name every time', () => {
    // The returning guest: they signed out as somebody, and the screen should
    // offer them that person again rather than a stranger.
    const first = guestNameFor(GUEST_ID);
    expect(guestNameFor(GUEST_ID)).toBe(first);
    expect(guestNameFor(GUEST_ID)).toBe(first);
  });

  it('is two words, and neither of them is Player', () => {
    const parts = guestNameFor(GUEST_ID).split(' ');
    expect(parts).toHaveLength(2);
    expect(parts).not.toContain('Player');
  });

  it('spreads different ids across the roster', () => {
    const seen = new Set<string>();
    for (let i = 0; i < 500; i++) seen.add(guestNameFor(`guest-${i}`));
    // Five hundred ids collapsing onto a handful of names would mean the hash
    // is folding, whatever the roster's size claims.
    expect(seen.size).toBeGreaterThan(100);
  });

  it('varies in both halves rather than moving them together', () => {
    // Two plain moduli of one number over two equally long lists would lock
    // the halves in step: 64 names wearing the costume of 4096.
    const adjectives = new Set<string>();
    const nouns = new Set<string>();
    for (let i = 0; i < 2000; i++) {
      const [adjective, noun] = guestNameFor(`guest-${i}`).split(' ');
      adjectives.add(adjective!);
      nouns.add(noun!);
    }
    expect(adjectives.size).toBeGreaterThan(20);
    expect(nouns.size).toBeGreaterThan(20);
    expect(adjectives.size * nouns.size).toBeLessThanOrEqual(GUEST_NAME_COUNT);
  });

  it('still invents a name with no id to work from', () => {
    // First run, before the server has issued a guest id. A random name is a
    // worse name than a derived one, and a much better one than everybody's.
    const seen = new Set<string>();
    for (let i = 0; i < 200; i++) seen.add(guestNameFor());
    expect(seen.size).toBeGreaterThan(20);
    expect(guestNameFor(null)).toEqual(expect.any(String));
  });

  it('agrees with the face the same id is dealt', () => {
    // Not a coincidence worth testing for its own sake — it is the promise the
    // guest screen makes, that the name and the face it offers are two views
    // of one device rather than two unrelated draws that can disagree after a
    // reload.
    const name = guestNameFor(GUEST_ID);
    const face = avatarFor(GUEST_ID, false).id;
    expect(guestNameFor(GUEST_ID)).toBe(name);
    expect(avatarFor(GUEST_ID, false).id).toBe(face);
  });
});
