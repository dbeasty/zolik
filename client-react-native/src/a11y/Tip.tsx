import {
  cloneElement,
  isValidElement,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactElement,
  type ReactNode,
} from 'react';
import {
  Platform,
  Pressable,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
  type StyleProp,
  type ViewStyle,
} from 'react-native';

import { visuallyHidden } from '@/src/a11y/LiveRegion';
import { getA11yPrefs, useA11yPrefs } from '@/src/a11y/prefs';

/**
 * Tooltips: the visible form of a control's accessible description.
 *
 * One text, three readers. A mouse user gets a bubble after resting on the
 * element; a keyboard user gets the same bubble the moment focus arrives; a
 * screen-reader user hears it as the element's description (`aria-describedby`
 * on the web, `accessibilityHint` on iOS and Android) without any gesture at
 * all. On a phone the bubble opens on a long-press, because a touch screen has
 * no hover.
 *
 * The bubble follows WCAG 1.4.13: it can be hovered without closing (the
 * pointer may move onto it), it stays until hover and focus have both left,
 * and Escape dismisses it. It is drawn by one `TipHost` at the root rather
 * than inside the element, so a panel's `overflow: hidden` never clips it.
 *
 * A tip is never the only place information lives. Anything a player needs to
 * act stays on screen; the tip explains it.
 */

type Shown = { id: string; text: string; x: number; y: number; w: number; h: number } | null;

let shown: Shown = null;
const listeners = new Set<() => void>();
let hideTimer: ReturnType<typeof setTimeout> | undefined;

function emit() {
  listeners.forEach((fn) => fn());
}

function showTip(next: NonNullable<Shown>) {
  if (hideTimer) clearTimeout(hideTimer);
  shown = next;
  emit();
}

/** Hides the tip if it is still the one `id` showed — a later tip is left alone. */
function hideTip(id?: string, delay = 0) {
  if (hideTimer) clearTimeout(hideTimer);
  const go = () => {
    if (id && shown?.id !== id) return;
    shown = null;
    emit();
  };
  if (delay) hideTimer = setTimeout(go, delay);
  else go();
}

/** Cancels a pending hide: the pointer moved from the element onto its bubble. */
function holdTip() {
  if (hideTimer) clearTimeout(hideTimer);
}

function subscribe(fn: () => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** For tests and for the shortcut sheet: whatever tip is open, closed. */
export function dismissTips() {
  hideTip();
}

/** Whether a bubble is showing — so Escape can close it before it means anything else. */
export function isTipOpen(): boolean {
  return shown !== null;
}

const HOVER_DELAY_MS = 500;
const TOUCH_SHOW_MS = 4000;

type TipProps = {
  /** What the bubble says, and what a screen reader hears as the description. */
  text: string | undefined | null;
  /** A keyboard shortcut to show after the text, e.g. "U". */
  shortcut?: string;
  /**
   * Make the wrapper itself a stop in the Tab order. For tips on things that
   * are not controls — a badge, a score — which a keyboard user could
   * otherwise never reach.
   */
  focusable?: boolean;
  /** The label a focusable wrapper is read as. Defaults to the text. */
  label?: string;
  testID?: string;
  /** For the wrapper — pass the child's flex if the wrapper must take its place in a row. */
  style?: StyleProp<ViewStyle>;
  children: ReactNode;
};

export function Tip({ text, shortcut, focusable, label, testID, style, children }: TipProps) {
  const id = `tip-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const ref = useRef<View>(null);
  const hoverTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const full = text ? (shortcut ? `${text} (${shortcut})` : text) : '';

  const open = useCallback(() => {
    if (!full || !getA11yPrefs().tooltips) return;
    const node = ref.current;
    if (!node?.measureInWindow) return;
    node.measureInWindow((x, y, w, h) => showTip({ id, text: full, x, y, w, h }));
  }, [full, id]);

  const close = useCallback(
    (delay = 0) => {
      if (hoverTimer.current) clearTimeout(hoverTimer.current);
      hideTip(id, delay);
    },
    [id],
  );

  useEffect(() => () => close(), [close]);

  if (!full) return <>{children}</>;

  // The description travels on the child itself, so whatever a screen reader
  // lands on — the button, not a wrapper around it — carries it.
  const only = isValidElement(children) ? (children as ReactElement<Record<string, unknown>>) : null;
  const childProps = only?.props ?? {};
  // An icon button's tip is usually its name ("Minimize Your hand"); linked
  // as its description too, a screen reader would say it twice.
  const sameAsName =
    !shortcut && (childProps.accessibilityLabel === text || childProps['aria-label'] === text);
  const described = sameAsName
    ? children
    : only
    ? cloneElement(only, {
        ...(Platform.OS === 'web'
          ? { 'aria-describedby': [childProps['aria-describedby'], id].filter(Boolean).join(' ') }
          : { accessibilityHint: (childProps.accessibilityHint as string | undefined) ?? full }),
        ...(Platform.OS !== 'web' && typeof childProps.onPress === 'function' && !childProps.onLongPress
          ? {
              onLongPress: () => {
                open();
                hideTip(id, TOUCH_SHOW_MS);
              },
            }
          : {}),
      })
    : children;

  const webHandlers =
    Platform.OS === 'web'
      ? {
          onPointerEnter: (e: { nativeEvent?: { pointerType?: string } }) => {
            if (e.nativeEvent?.pointerType && e.nativeEvent.pointerType !== 'mouse') return;
            if (hoverTimer.current) clearTimeout(hoverTimer.current);
            hoverTimer.current = setTimeout(open, HOVER_DELAY_MS);
          },
          onPointerLeave: () => close(150),
          onFocus: (e: { target?: unknown }) => {
            // Only a keyboard's focus: a mouse click focuses too, and a
            // bubble on every click is noise.
            const el = e.target as { matches?: (s: string) => boolean } | undefined;
            if (el?.matches && !el.matches(':focus-visible')) return;
            open();
          },
          onBlur: () => close(),
        }
      : {};

  // A child that is not itself pressable gets one, on native, so a long-press
  // still opens the bubble — a badge has no press of its own to borrow.
  const needsPress = Platform.OS !== 'web' && typeof childProps.onPress !== 'function';

  const body = needsPress ? (
    <Pressable
      accessible={!!focusable}
      accessibilityLabel={focusable ? (label ?? full) : undefined}
      accessibilityHint={focusable && label ? full : undefined}
      onLongPress={() => {
        open();
        hideTip(id, TOUCH_SHOW_MS);
      }}
      delayLongPress={350}
    >
      {described}
    </Pressable>
  ) : (
    described
  );

  return (
    <View
      ref={ref}
      testID={testID}
      collapsable={false}
      style={focusable ? [style, styles.focusTarget] : style}
      {...(Platform.OS === 'web' && focusable
        ? { tabIndex: 0, role: 'note', 'aria-label': label ?? full, 'aria-describedby': label ? id : undefined }
        : {})}
      {...(webHandlers as object)}
    >
      {body}
      {Platform.OS === 'web' ? (
        // Hidden from the tree, still the description: `aria-describedby`
        // reads hidden content by design, and without this a screen reader
        // reading the page in order would say every tip a second time as a
        // loose line of text after the control it belongs to.
        <View nativeID={id} style={visuallyHidden} aria-hidden>
          <Text>{full}</Text>
        </View>
      ) : null}
    </View>
  );
}

/**
 * The one bubble, drawn over everything. Mounted once at the root, beside the
 * live region; renders nothing until a `Tip` asks.
 */
export function TipHost() {
  const tip = useSyncExternalStore(subscribe, () => shown, () => null);
  const prefs = useA11yPrefs();
  const { width, height } = useWindowDimensions();
  const [size, setSize] = useState({ w: 0, h: 0 });

  useEffect(() => {
    if (Platform.OS !== 'web' || !tip) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') hideTip();
    };
    const onScroll = () => hideTip();
    document.addEventListener('keydown', onKey);
    window.addEventListener('scroll', onScroll, true);
    return () => {
      document.removeEventListener('keydown', onKey);
      window.removeEventListener('scroll', onScroll, true);
    };
  }, [tip]);

  if (!tip || !prefs.tooltips) return null;

  // Above the element where there is room, below it where there is not;
  // centred on it, and kept inside the window with an 8px margin.
  const margin = 8;
  const above = tip.y - size.h - 6 >= margin;
  const top = above ? tip.y - size.h - 6 : Math.min(tip.y + tip.h + 6, height - size.h - margin);
  const left = Math.max(margin, Math.min(tip.x + tip.w / 2 - size.w / 2, width - size.w - margin));

  return (
    <View
      testID="tooltip"
      // Hoverable: moving onto the bubble keeps it open (WCAG 1.4.13).
      {...((Platform.OS === 'web'
        ? { onPointerEnter: holdTip, onPointerLeave: () => hideTip(tip.id, 150), role: 'tooltip', 'aria-hidden': true }
        : { importantForAccessibility: 'no-hide-descendants', accessibilityElementsHidden: true }) as object)}
      onLayout={(e) => {
        const { width: w, height: h } = e.nativeEvent.layout;
        if (w !== size.w || h !== size.h) setSize({ w, h });
      }}
      style={[
        styles.bubble,
        Platform.OS === 'web' ? ({ position: 'fixed' } as object) : null,
        { top, left, opacity: size.w ? 1 : 0 },
      ]}
    >
      <Text style={styles.text}>{tip.text}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  // A focusable wrapper is something a finger can land on: 24px each way (WCAG 2.2 2.5.8),
  // whatever 12px line of text it happens to carry.
  focusTarget: { minHeight: 24, justifyContent: 'center' },
  bubble: {
    position: 'absolute',
    zIndex: 10000,
    maxWidth: 280,
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 6,
    backgroundColor: '#0b0f14',
    borderWidth: 1,
    borderColor: '#e8eef5',
    shadowColor: '#000',
    shadowOpacity: 0.4,
    shadowRadius: 6,
    shadowOffset: { width: 0, height: 2 },
    elevation: 12,
  },
  text: { color: '#ffffff', fontSize: 13, lineHeight: 18 },
});
