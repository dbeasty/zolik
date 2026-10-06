import { useWindowDimensions } from 'react-native';

/** Where the menu and a game's page stop being one column and grow a side panel. */
export const WIDE_MIN_WIDTH = 900;

/** Whether the window is wide enough for the two-column menu layouts. */
export function useWide(): boolean {
  return useWindowDimensions().width >= WIDE_MIN_WIDTH;
}
