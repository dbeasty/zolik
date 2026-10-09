import { useEffect, useRef, type ReactNode } from 'react';
import { Modal, Platform, Pressable, View, type StyleProp, type ViewStyle } from 'react-native';

import { webAttrs } from '@/src/a11y/props';

/**
 * A panel over the screen — a menu, a picker, a list — that behaves as a
 * dialog for everyone who cannot see it open.
 *
 * React Native's `Modal` gets most of the way on its own: react-native-web
 * renders it as `role="dialog" aria-modal`, traps Tab inside, closes on
 * Escape through `onRequestClose`, and puts focus back on the opener when it
 * closes; Android's back button calls the same `onRequestClose`. What it does
 * not do, and this does:
 *
 * - **Name the dialog**, so it is announced as "Account, dialog" rather than
 *   as "dialog".
 * - **Move focus in on open** (web). Left alone, the trap focuses the first
 *   thing it finds — the backdrop, a full-screen button with no name.
 * - **Keep the backdrop out of the way.** The tap-outside-to-close backdrop is
 *   a `Pressable` around the panel; as a focusable, accessible element it was
 *   a nameless Tab stop on the web and, on iOS, swallowed the whole panel into
 *   one VoiceOver element. It is now neither: pointer-only, as it is meant.
 * - **Confine VoiceOver** to the panel (`accessibilityViewIsModal`) and let
 *   its escape gesture close it (`onAccessibilityEscape`).
 */
export function Sheet({
  visible,
  onClose,
  label,
  animationType = 'fade',
  backdropStyle,
  style,
  backdropTestID,
  testID,
  children,
}: {
  visible: boolean;
  onClose: () => void;
  /** What the dialog is called — usually the panel's own heading. */
  label: string;
  animationType?: 'none' | 'fade' | 'slide';
  backdropStyle: StyleProp<ViewStyle>;
  style: StyleProp<ViewStyle>;
  backdropTestID?: string;
  testID?: string;
  children: ReactNode;
}) {
  const panel = useRef<View>(null);

  useEffect(() => {
    if (!visible || Platform.OS !== 'web') return;
    // After the trap's own first pass, which runs once the modal has mounted.
    const timer = setTimeout(() => focusFirstIn(panel.current as unknown as HTMLElement | null), 0);
    return () => clearTimeout(timer);
  }, [visible]);

  return (
    <Modal
      transparent
      animationType={animationType}
      visible={visible}
      onRequestClose={onClose}
      {...(webAttrs({ 'aria-label': label }) as object)}
    >
      <Pressable
        style={backdropStyle}
        onPress={onClose}
        testID={backdropTestID}
        accessible={false}
        importantForAccessibility="no"
        tabIndex={-1}
      >
        {/* A pressable that does nothing, so a press inside the panel does
            not reach the backdrop and close it. Not a control itself. */}
        <Pressable
          ref={panel}
          style={style}
          onPress={() => {}}
          testID={testID}
          accessible={false}
          tabIndex={-1}
          accessibilityViewIsModal
          onAccessibilityEscape={onClose}
        >
          {children}
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const FOCUSABLE =
  'button, a[href], input, select, textarea, [tabindex="0"], [role="button"], [role="radio"], [role="link"], [role="switch"], [role="checkbox"]';

/** Focuses the first control in `root`, or `root` itself when it has none. */
export function focusFirstIn(root: HTMLElement | null) {
  if (!root || typeof document === 'undefined') return;
  if (root.contains(document.activeElement) && document.activeElement !== root) return;
  const first = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).find(
    (el) => el.getAttribute('aria-disabled') !== 'true' && el.tabIndex >= 0,
  );
  (first ?? root).focus();
}
