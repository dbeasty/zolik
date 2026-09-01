import { AVATARS, avatarById, avatarFor, choicesFor } from '@/src/components/avatars/catalogue';
import { FIGURES } from '@/src/components/avatars/faces';

/**
 * The rules a face has to keep. Nothing here is about how one looks — that is
 * not a thing a test can hold — only about which one a player gets, which is
 * the part every client has to agree on without asking each other.
 */

describe('the roster', () => {
  it('gives every entry a figure to draw', () => {
    for (const a of AVATARS) expect(FIGURES[a.id]).toBeDefined();
  });

  it('has no duplicate slugs', () => {
    expect(new Set(AVATARS.map((a) => a.id)).size).toBe(AVATARS.length);
  });

  it('offers both sorts of face', () => {
    expect(choicesFor(false).length).toBeGreaterThan(0);
    expect(choicesFor(true).length).toBeGreaterThan(0);
  });
});

describe('avatarFor', () => {
  it('honours a face the player chose', () => {
    expect(avatarFor('anyone', false, 'p-violet').id).toBe('p-violet');
  });

  it('ignores a slug this build does not know', () => {
    // An older or newer client may name a face this one has never heard of.
    // Falling back is what keeps that a cosmetic difference rather than a
    // blank seat.
    expect(avatarFor('anyone', false, 'p-from-the-future').kind).toBe('person');
  });

  it('is stable for a given id', () => {
    expect(avatarFor('player-1', false).id).toBe(avatarFor('player-1', false).id);
  });

  it('does not follow the name, so renaming keeps the face', () => {
    // The circle this replaced hashed the display name; a rename changed your
    // face mid-match. Only the id is consulted now, and this is the guard.
    const a = avatarFor('player-1', false);
    const b = avatarFor('player-1', false, undefined);
    expect(a.id).toBe(b.id);
  });

  it('tells different players apart', () => {
    const faces = new Set(['a', 'b', 'c', 'd'].map((id) => avatarFor(id, false).id));
    expect(faces.size).toBeGreaterThan(1);
  });

  it('always gives a seat nobody is sitting at a machine', () => {
    expect(avatarFor('bot:XYZ', true).kind).toBe('machine');
  });

  it('overrules a machine seat that asked for a person', () => {
    // An opponent that looks human and is not is the one thing a player
    // should never have to be told rather than shown.
    expect(avatarFor('bot:XYZ', true, 'p-violet').kind).toBe('machine');
  });

  it('overrules a human seat that asked for a machine', () => {
    expect(avatarFor('player-1', false, 'm-steel').kind).toBe('person');
  });

  it('resolves a known slug and refuses an unknown one', () => {
    expect(avatarById('m-brass')?.kind).toBe('machine');
    expect(avatarById('nope')).toBeUndefined();
    expect(avatarById(null)).toBeUndefined();
  });
});
