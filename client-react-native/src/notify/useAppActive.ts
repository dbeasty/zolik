import { useEffect, useState } from 'react';
import { AppState } from 'react-native';

/**
 * Whether the app is in front. The personal socket and the nearby watcher
 * both stop the moment it is not: in the background the OS push is the
 * channel, and a radio scanning for a player who is not looking is only
 * spending their battery.
 */
export function useAppActive(): boolean {
  const [active, setActive] = useState(() => AppState.currentState !== 'background');
  useEffect(() => {
    const sub = AppState.addEventListener('change', (s) => setActive(s === 'active'));
    return () => sub.remove();
  }, []);
  return active;
}
