import type { ReactNode } from 'react';
import { Platform, View, type StyleProp, type ViewStyle } from 'react-native';

import { webAttrs } from '@/src/a11y/props';

/**
 * A row of mutually exclusive choices, as a screen reader and a keyboard
 * expect one: a named `radiogroup` whose children are `radio`s carrying
 * `aria-checked`.
 *
 * The radios themselves stay ordinary `Pressable`s at each call site, with
 * `role="radio"` and `aria-checked` — see `radioProps` — so every picker keeps
 * its own look. What this adds is the group's name (so "Easy, radio button,
 * 2 of 3" is heard as part of "Opponents") and, on the web, the arrow keys:
 * ←/→ and ↑/↓ move to the neighbouring choice and pick it, which is how a
 * radio group behaves everywhere else on the web. Tab still visits each one,
 * so nobody has to know about the arrows.
 */
export function RadioGroup({
  label,
  labelledBy,
  style,
  testID,
  children,
}: {
  /** The group's name, when there is no visible label to point at. */
  label?: string;
  /** The `nativeID` of the visible text naming the group. */
  labelledBy?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  children: ReactNode;
}) {
  return (
    <View
      testID={testID}
      style={style}
      role="radiogroup"
      aria-label={labelledBy ? undefined : label}
      accessibilityLabel={label}
      aria-labelledby={labelledBy}
      {...webAttrs({ onKeyDown: onArrow })}
    >
      {children}
    </View>
  );
}

/** The props that make a `Pressable` one of a `RadioGroup`'s choices. */
export function radioProps(checked: boolean, label?: string) {
  return {
    role: 'radio' as const,
    'aria-checked': checked,
    // Read by native when `aria-checked` alone is not mapped (older
    // architectures), ignored by react-native-web, which reads the aria form.
    accessibilityState: { checked },
    ...(label ? { 'aria-label': label } : {}),
  };
}

const KEYS: Record<string, number> = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 };

function onArrow(e: { key: string; currentTarget: unknown; target: unknown; preventDefault: () => void }) {
  if (Platform.OS !== 'web') return;
  const step = KEYS[e.key];
  if (!step) return;
  const group = e.currentTarget as HTMLElement;
  const radios = Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]')).filter(
    (r) => r.getAttribute('aria-disabled') !== 'true',
  );
  const at = radios.indexOf(document.activeElement as HTMLElement);
  if (at === -1) return;
  e.preventDefault();
  const next = radios[(at + step + radios.length) % radios.length];
  next?.focus();
  // Arrowing onto a radio picks it, as a native radio group does.
  next?.click();
}
