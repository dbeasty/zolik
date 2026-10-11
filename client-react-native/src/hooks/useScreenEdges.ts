import { usePathname } from 'expo-router';
import type { Edge } from 'react-native-safe-area-context';

/** Routes the root stack draws without a header (see app/_layout.tsx). */
const HEADERLESS = ['/auth/callback'];

/**
 * Which edges a screen's SafeAreaView should inset.
 *
 * Every screen sits below the stack's header, which already clears the status
 * bar and the notch. Insetting the top again left a band of dead space as tall
 * as the notch (62 pt on an iPhone 17 Pro) between the header and the page,
 * on every screen. Only a screen with no header of its own needs the top.
 */
export function screenEdges(pathname: string): Edge[] {
  return HEADERLESS.some((p) => pathname.startsWith(p)) ? ['top', 'left', 'right'] : ['left', 'right'];
}

export function useScreenEdges(): Edge[] {
  return screenEdges(usePathname());
}
