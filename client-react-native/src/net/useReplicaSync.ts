import { useEffect } from 'react';
import { AppState } from 'react-native';

import * as localNode from '@/src/net/localNode';

/**
 * When this device replicates.
 *
 * The replicator syncs on its own every minute and whenever something is
 * written, which is enough for correctness and not enough for the moments a
 * person is actually looking: coming back to the app expecting to see the
 * match they finished on another device, or watching their history after the
 * signal came back. Those are the ones worth asking for.
 *
 * Failures are deliberately silent. A sync that could not happen is something
 * to try again at the next of these moments, not an error to put in front of
 * somebody who is looking at their scorepads.
 */
export function useReplicaSync(enabled: boolean): void {
  useEffect(() => {
    if (!enabled) return;
    void localNode.sync();

    const sub = AppState.addEventListener('change', (next) => {
      if (next === 'active') void localNode.sync();
    });
    return () => sub.remove();
  }, [enabled]);
}
