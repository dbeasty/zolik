import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { avatarById } from '@/src/components/avatars/catalogue';
import { useSession } from '@/src/context/SessionContext';
import { loadAvatarId, saveAvatarId } from '@/src/lib/avatarStore';

/**
 * Which face the player wears, shared down the tree the same way `useSkin`
 * shares the look.
 *
 * Two places remember it, and the split is the whole design:
 *
 * - **The device**, always. It is what a guest has instead of an account, and
 *   it is what puts the right face on screen in the first frame after a
 *   reload, before anything has been fetched.
 * - **The account**, when there is one, through the preferences that already
 *   round-trip on `/users/me`. That is what makes it follow somebody to a new
 *   device rather than living in one browser's storage.
 *
 * The account wins when the two disagree, because the account is the thing
 * that travelled and the device is the thing that was left behind.
 *
 * Nothing here decides what a *seat* shows. That is `avatarFor`, which falls
 * back to deriving one from the player id — this hook only answers "what did
 * this person choose", and the answer is allowed to be nothing.
 */

type AvatarState = {
  /** The chosen slug, or null when they have never picked one. */
  avatarId: string | null;
  setAvatarId: (id: string) => void;
};

const AvatarContext = createContext<AvatarState | null>(null);

export function AvatarProvider({ children }: { children: ReactNode }) {
  const { account, client, session } = useSession();
  const [avatarId, setAvatar] = useState<string | null>(null);

  // The device's answer, a tick after first render — same trade the skin
  // makes, and invisible for the same reason: one frame of no face is
  // cheaper than holding the whole app behind a preference read.
  useEffect(() => {
    let live = true;
    loadAvatarId().then((id) => {
      if (live && avatarById(id)) setAvatar(id);
    });
    return () => {
      live = false;
    };
  }, []);

  // The account's answer, whenever one arrives, overriding the device's. Also
  // written back to the device, so the next reload starts from the right face
  // rather than from whatever this browser last happened to hold.
  useEffect(() => {
    const fromAccount = account?.prefs?.avatar;
    if (!fromAccount || !avatarById(fromAccount)) return;
    setAvatar(fromAccount);
    void saveAvatarId(fromAccount);
  }, [account?.prefs?.avatar]);

  const setAvatarId = useCallback(
    (id: string) => {
      if (!avatarById(id)) return;
      setAvatar(id);
      void saveAvatarId(id);
      // Signed in, so it should outlive this device. A failure here is not
      // worth surfacing: the choice already took effect locally, and the
      // account will catch up the next time they change it.
      if (session && !session.isGuest) {
        void client
          .updateMe({ preferences: { ...(account?.prefs ?? {}), avatar: id } })
          .catch(() => {});
      }
    },
    [account?.prefs, client, session],
  );

  // The client carries the face to every seat this player takes, so it has to
  // know the current one before any of them is taken — see `ZolikClient.avatarId`.
  useEffect(() => {
    client.avatarId = avatarId ?? '';
  }, [avatarId, client]);

  const value = useMemo<AvatarState>(() => ({ avatarId, setAvatarId }), [avatarId, setAvatarId]);
  return <AvatarContext.Provider value={value}>{children}</AvatarContext.Provider>;
}

/**
 * The chosen face, or null. Falls back to null outside a provider so a
 * component rendered in a test without one still works — a seat with no
 * choice is an ordinary state, not an error.
 */
export function useAvatarId(): string | null {
  return useContext(AvatarContext)?.avatarId ?? null;
}

/** The picker's view: the choice, and the setter. */
export function useAvatarControls(): AvatarState {
  return useContext(AvatarContext) ?? { avatarId: null, setAvatarId: () => {} };
}
