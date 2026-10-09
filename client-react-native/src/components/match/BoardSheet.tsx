import { useEffect, useMemo, useRef, type ReactNode } from 'react';
import { Modal, Platform, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { useSkin } from '@/src/hooks/useSkin';
import type { Skin } from '@/src/skins/types';

/**
 * A small question the board asks, as a sheet of large buttons: where to play
 * a card that fits more than one place, whether to make a move that ends the
 * hand, which keys do what.
 *
 * The keyboard's half matters as much as the picture's. The first button has
 * focus the moment the sheet opens (a sheet that opened behind the focus would
 * leave a keyboard player pressing Tab through the board to find it), Escape
 * closes it (`Modal` hears it on the web), and it is a dialog with a name, so
 * a screen reader says what was asked before it says the first answer.
 */
export type SheetOption = {
  key: string;
  label: string;
  testID?: string;
  /** Drawn as the suggested answer. */
  primary?: boolean;
  onPress: () => void;
};

export function BoardSheet({
  visible,
  title,
  body,
  options,
  cancelLabel,
  onCancel,
  testID,
  children,
}: {
  visible: boolean;
  title: string;
  body?: string;
  options: SheetOption[];
  cancelLabel: string;
  onCancel: () => void;
  testID: string;
  /** Anything the sheet shows that is not an answer — the shortcut list. */
  children?: ReactNode;
}) {
  const skin = useSkin();
  const styles = useMemo(() => sheetStyles(skin), [skin]);
  const first = useRef<View>(null);

  useEffect(() => {
    if (!visible || Platform.OS !== 'web') return;
    // After the modal has mounted its content; a focus asked for in the same
    // tick lands on a node that is not in the document yet.
    const timer = setTimeout(() => (first.current as unknown as { focus?: () => void } | null)?.focus?.(), 30);
    return () => clearTimeout(timer);
  }, [visible]);

  if (!visible) return null;
  return (
    <Modal
      transparent
      animationType="none"
      visible
      onRequestClose={onCancel}
      // The web's Modal is already the dialog (`role="dialog"`, `aria-modal`);
      // this names it, so the question is said before the first answer.
      {...((Platform.OS === 'web' ? { 'aria-label': title } : {}) as object)}
    >
      <Pressable style={styles.scrim} onPress={onCancel} testID={`${testID}-scrim`} accessible={false}>
        <Pressable
          style={styles.sheet}
          testID={testID}
          onPress={() => undefined}
          accessible={false}
          {...((Platform.OS === 'web' ? {} : { accessibilityViewIsModal: true }) as object)}
        >
          <Text style={styles.title} accessibilityRole="header">
            {title}
          </Text>
          {body ? <Text style={styles.body}>{body}</Text> : null}
          {children ? <ScrollView style={styles.scroll}>{children}</ScrollView> : null}
          <View style={styles.options}>
            {options.map((o, i) => (
              <Pressable
                key={o.key}
                ref={i === 0 ? first : undefined}
                testID={o.testID}
                accessibilityRole="button"
                onPress={o.onPress}
                style={({ pressed }) => [styles.option, o.primary && styles.primary, pressed && styles.pressed]}
              >
                <Text style={[styles.optionText, o.primary && styles.primaryText]}>{o.label}</Text>
              </Pressable>
            ))}
          </View>
          <Pressable
            ref={options.length ? undefined : first}
            onPress={onCancel}
            testID={`${testID}-cancel`}
            accessibilityRole="button"
            style={styles.cancel}
          >
            <Text style={styles.cancelText}>{cancelLabel}</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

function sheetStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    scrim: { flex: 1, backgroundColor: 'rgba(0,0,0,0.55)', justifyContent: 'center', alignItems: 'center', padding: 16 },
    sheet: {
      width: '100%',
      maxWidth: 460,
      maxHeight: '90%',
      backgroundColor: colors.surface,
      borderColor: colors.accent,
      borderWidth: 1,
      borderRadius: 14,
      padding: 18,
      gap: 12,
    },
    title: { color: colors.text, fontSize: 18, fontWeight: '700', textAlign: 'center' },
    body: { color: colors.text, fontSize: 15, textAlign: 'center' },
    scroll: { maxHeight: 360 },
    options: { gap: 8 },
    option: {
      minHeight: 44,
      paddingVertical: 12,
      paddingHorizontal: 14,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.bg,
      justifyContent: 'center',
    },
    primary: { backgroundColor: colors.accentButton, borderColor: colors.accentButton },
    pressed: { opacity: 0.85 },
    optionText: { color: colors.text, fontSize: 16, fontWeight: '600' },
    primaryText: { color: colors.onAccent },
    cancel: { alignSelf: 'center', minHeight: 44, justifyContent: 'center', paddingHorizontal: 16 },
    cancelText: { color: colors.muted, fontSize: 15 },
  });
}
