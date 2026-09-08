import { codeFromInviteInput, INVITE_PATH, inviteUrlFor } from '@/src/lib/inviteLink';

/**
 * The link is the feature, so these are the tests that matter most: a wrong
 * answer here is a URL pasted into somebody's chat that goes nowhere, and the
 * person who finds out is the guest, on a different device, with no way to
 * report what they saw.
 *
 * `inviteUrlFor` takes its origin as an argument precisely so this file can
 * state the browser/native split as data instead of faking a `window`.
 */

const SERVER_LINK = 'https://play.limidus.com/join/ABC123';

describe('inviteUrlFor', () => {
  it('uses the server-minted link when there is no browser origin', () => {
    // The native case: a phone has no origin of its own, and the server's
    // public address is the only thing that is true for the recipient.
    expect(inviteUrlFor({ joinCode: 'ABC123', inviteUrl: SERVER_LINK }, '')).toBe(SERVER_LINK);
  });

  it('prefers the origin the client is actually served from', () => {
    // Development: Expo serves the client on :8114 while the server's
    // configured public base names :8090, where no client answers. Trusting
    // the server there hands out a link to a blank page — which is exactly
    // the bug this ordering exists to prevent.
    expect(inviteUrlFor({ joinCode: 'ABC123', inviteUrl: 'http://localhost:8090/join/ABC123' }, 'http://localhost:8114'))
      .toBe('http://localhost:8114/join/ABC123');
  });

  it('keeps the path the server named when swapping the origin', () => {
    // The route belongs to the server (match.InvitePath). A client that
    // rebuilt the path from its own constant would mint dead links the moment
    // the two disagreed, so only the origin is replaced.
    expect(inviteUrlFor({ joinCode: 'X', inviteUrl: 'https://prod.example/invite/v2/X' }, 'http://localhost:8114'))
      .toBe('http://localhost:8114/invite/v2/X');
  });

  it('builds a link from the code when the server sent none', () => {
    // An older server, or one with no public base configured. On web the
    // origin is known for certain, so a link is still the right offer.
    expect(inviteUrlFor({ joinCode: 'ABC123' }, 'https://play.limidus.com')).toBe(
      `https://play.limidus.com${INVITE_PATH}ABC123`,
    );
  });

  it('offers nothing rather than a broken link', () => {
    // No code, no server link, no origin: every ingredient missing. A "Copy
    // link" button that copies "undefined/join/" is worse than no button, and
    // the panel keys its fallback text off exactly this empty string.
    expect(inviteUrlFor({}, '')).toBe('');
    expect(inviteUrlFor({ joinCode: '' }, 'https://play.limidus.com')).toBe('');
  });

  it('leaves a link it cannot parse alone', () => {
    // Safe direction: the server's own answer is a worse guess than a
    // rewritten one, but never a broken one.
    expect(inviteUrlFor({ inviteUrl: 'not a url at all' }, 'https://play.limidus.com')).toBe(
      'not a url at all',
    );
  });

  it('tolerates a trailing slash on the origin', () => {
    expect(inviteUrlFor({ joinCode: 'ABC123', inviteUrl: SERVER_LINK }, 'https://other.example/')).toBe(
      'https://other.example/join/ABC123',
    );
  });
});

describe('codeFromInviteInput', () => {
  it('pulls the code out of a pasted link', () => {
    expect(codeFromInviteInput(SERVER_LINK)).toBe('ABC123');
  });

  it('survives what a chat client sticks on the end', () => {
    // Query strings from link trackers, fragments, a trailing slash, and the
    // sentence the link was pasted in the middle of.
    expect(codeFromInviteInput('https://play.limidus.com/join/ABC123?utm=chat')).toBe('ABC123');
    expect(codeFromInviteInput('https://play.limidus.com/join/ABC123#top')).toBe('ABC123');
    expect(codeFromInviteInput('https://play.limidus.com/join/ABC123/')).toBe('ABC123');
    expect(codeFromInviteInput('  https://play.limidus.com/join/ABC123 come play  ')).toBe('ABC123');
  });

  it('passes a bare code through untouched', () => {
    // The box has always taken a code or a match id, and must keep doing so —
    // accepting links is an addition, not a replacement.
    expect(codeFromInviteInput('ABC123')).toBe('ABC123');
    expect(codeFromInviteInput('  ABC123  ')).toBe('ABC123');
    expect(codeFromInviteInput('68b0f2c1a4e3d90012345678')).toBe('68b0f2c1a4e3d90012345678');
  });

  it('reads the last /join/ in a link, not the first', () => {
    // A host whose deployment lives under a path prefix, or anything else
    // that puts the word twice in one URL: the code is what follows the last
    // one, because that is the segment the route matched.
    expect(codeFromInviteInput('https://example.com/join/games/join/ZZ9000')).toBe('ZZ9000');
  });

  it('decodes an escaped code', () => {
    expect(codeFromInviteInput('https://play.limidus.com/join/AB%20C1')).toBe('AB C1');
  });

  it('returns the raw text rather than throwing on a mangled escape', () => {
    expect(codeFromInviteInput('https://play.limidus.com/join/AB%ZZ')).toBe('AB%ZZ');
  });

  it('gives back nothing for an empty box', () => {
    expect(codeFromInviteInput('   ')).toBe('');
  });
});
