import { router, useGlobalSearchParams, usePathname } from 'expo-router';
import { useEffect, useState, useSyncExternalStore } from 'react';
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native';

import { Tip } from '@/src/a11y/Tip';
import { t } from '@/src/lib/i18n';
import { aheadCount, leavingBack, routeChanged, subscribeAhead, takeForward } from '@/src/lib/navHistory';
import { colors } from '@/src/theme';

type NavState = { canGoBack: boolean; canGoForward: boolean };
type DesktopNav = {
  state(): NavState;
  subscribe(cb: (s: NavState) => void): () => void;
  go(dir: 'back' | 'forward'): void;
};

function desktopNav(): DesktopNav | null {
  return (globalThis as { ZolikDesktopNav?: DesktopNav }).ZolikDesktopNav ?? null;
}

/**
 * Back and forward, in every header: going back always has a way forward, so
 * a player who goes from a game to the main screen has an arrow right there
 * to return.
 *
 * In the Mac app they walk the window's own history, the same one View ›
 * Back and Forward do. Elsewhere the stack goes back, and the screens it
 * left are kept for → (src/lib/navHistory.ts).
 */
export function NavArrows() {
  return desktopNav() ? <DesktopArrows /> : <StackArrows />;
}

/** Where this screen is, as a route the forward arrow can return to. */
function useCurrentRoute(): string {
  const pathname = usePathname();
  const params = useGlobalSearchParams();
  if (Platform.OS === 'web' && typeof window !== 'undefined') return window.location.pathname + window.location.search;
  // A dynamic segment's value is already in the path; only the rest are query.
  const segments = new Set(pathname.split('/').filter(Boolean));
  const query = Object.entries(params)
    .filter(([, v]) => typeof v === 'string' && !segments.has(v))
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
    .join('&');
  return query ? `${pathname}?${query}` : pathname;
}

/**
 * Tells the forward list about every route change, once: mounted in the root
 * layout, while every screen's header draws its own arrows.
 */
export function NavHistoryTracker() {
  const route = useCurrentRoute();
  const [first, setFirst] = useState(true);
  useEffect(() => {
    if (first) {
      setFirst(false);
      return;
    }
    routeChanged();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [route]);
  return null;
}

function StackArrows() {
  const route = useCurrentRoute();
  const ahead = useSyncExternalStore(subscribeAhead, aheadCount, aheadCount);
  return (
    <View style={styles.row}>
      <Arrow
        glyph="←"
        label={t('desktop.menu.back')}
        enabled={router.canGoBack()}
        testID="nav-back"
        onPress={() => {
          leavingBack(route);
          router.back();
        }}
      />
      <Arrow
        glyph="→"
        label={t('desktop.menu.forward')}
        enabled={ahead > 0}
        testID="nav-forward"
        onPress={() => {
          const next = takeForward();
          if (next) router.push(next as never);
        }}
      />
    </View>
  );
}

function DesktopArrows() {
  const nav = desktopNav();
  const [state, setState] = useState<NavState>(() => nav?.state() ?? { canGoBack: false, canGoForward: false });

  useEffect(() => nav?.subscribe(setState), [nav]);

  if (!nav) return null;
  return (
    <View style={styles.row}>
      <Arrow glyph="←" label={t('desktop.menu.back')} enabled={state.canGoBack} testID="nav-back" onPress={() => nav.go('back')} />
      <Arrow
        glyph="→"
        label={t('desktop.menu.forward')}
        enabled={state.canGoForward}
        testID="nav-forward"
        onPress={() => nav.go('forward')}
      />
    </View>
  );
}

function Arrow(props: { glyph: string; label: string; enabled: boolean; testID: string; onPress: () => void }) {
  return (
    <Tip text={props.label}>
      <Pressable
        testID={props.testID}
        role="button"
        aria-label={props.label}
        // react-native-web's Pressable sets aria-disabled from this itself.
        disabled={!props.enabled}
        onPress={props.onPress}
        hitSlop={6}
        style={styles.arrow}
      >
        <Text style={[styles.glyph, !props.enabled && styles.off]}>{props.glyph}</Text>
      </Pressable>
    </Tip>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 4, marginLeft: 8, marginRight: 8 },
  arrow: { paddingHorizontal: 8, paddingVertical: 4, borderRadius: 6 },
  glyph: { color: colors.text, fontSize: 22, lineHeight: 26 },
  off: { opacity: 0.3 },
});
