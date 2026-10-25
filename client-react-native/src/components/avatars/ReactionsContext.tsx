import { createContext, useContext } from 'react';

import type { Reaction } from '@/src/components/avatars/reactions';

/**
 * The reactions the match screen is playing, by player id.
 *
 * A context rather than a prop because the faces that react sit several
 * layouts down from the screen that hears the board change (screen →
 * `BoardLayout` → `SeatStrip`), and nothing in between has any business with
 * them. Outside a match there is no provider, and every face holds still.
 */
export const ReactionsContext = createContext<Record<string, Reaction>>({});

/** Every reaction in play, for a component drawing several faces at once. */
export function useReactions(): Record<string, Reaction> {
  return useContext(ReactionsContext);
}
