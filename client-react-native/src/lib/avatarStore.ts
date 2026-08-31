import { storage } from '@/src/context/SessionContext';

/**
 * The face this device picked, remembered on it.
 *
 * The same shape as the skin's preference (`src/skins/index.ts`) and for the
 * same reason: a guest has no account to hang a choice on, and a choice that
 * did not survive a reload would not be a choice so much as a per-visit
 * accident.
 *
 * A signed-in player's face lives on their account instead, so it follows them
 * to a new device — but it is written here as well, which is what lets the
 * board show the right face on the very first frame after a reload, before the
 * account has been fetched.
 *
 * Kept across sign-out, like the guest identity beside it: signing out is not
 * a request to stop looking like yourself.
 */
const KEY = 'zolik_avatar';

export async function loadAvatarId(): Promise<string | null> {
  try {
    return await storage.getItem(KEY);
  } catch {
    return null;
  }
}

export async function saveAvatarId(id: string): Promise<void> {
  try {
    await storage.setItem(KEY, id);
  } catch {
    // Losing a preference is a small thing; interrupting a game over it is not.
  }
}
