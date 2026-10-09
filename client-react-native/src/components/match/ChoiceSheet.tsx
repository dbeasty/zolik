import { useMemo } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { ParamSpec } from '@/src/api/matchTypes';
import { Sheet } from '@/src/a11y/Sheet';
import { t } from '@/src/lib/i18n';
import { label } from '@/src/lib/labels';
import { useSkin } from '@/src/hooks/useSkin';
import type { Skin } from '@/src/skins/types';

/**
 * The question a card asks before it goes — which colour a wild names — as a
 * sheet of large buttons, one per answer.
 *
 * Opened by the match screen whenever a move plays a card the offer says a
 * choice is for (`ParamSpec.cards`), however the card was played: a dragged
 * card has no control beside it to set the choice on, and a choice nobody
 * was asked for is a choice the player never made. Answering sends the move;
 * dismissing sends nothing, and the card stays in hand.
 *
 * The question and the answers are the module's own keys.
 */
export function ChoiceSheet({
  spec,
  onPick,
  onCancel,
}: {
  spec: ParamSpec | null;
  onPick: (name: string, value: string) => void;
  onCancel: () => void;
}) {
  const skin = useSkin();
  const styles = useMemo(() => choiceStyles(skin), [skin]);
  if (!spec) return null;

  return (
    <Sheet
      visible
      onClose={onCancel}
      label={label(spec.labelKey)}
      backdropStyle={styles.scrim}
      backdropTestID="choice-scrim"
      style={styles.sheet}
      testID={`choice-${spec.name}`}
    >
      <Text style={styles.title}>{label(spec.labelKey)}</Text>
      <View style={styles.grid}>
        {(spec.choices ?? []).map((c) => (
          <Pressable
            key={c.value}
            testID={`choice-${spec.name}-${c.value}`}
            accessibilityRole="button"
            onPress={() => onPick(spec.name, c.value)}
            style={({ pressed }) => [
              styles.option,
              c.value === spec.defaultChoice && styles.suggested,
              pressed && styles.pressed,
            ]}
          >
            <Text style={styles.optionText}>{label(c.labelKey)}</Text>
          </Pressable>
        ))}
      </View>
      <Pressable onPress={onCancel} testID="choice-cancel" accessibilityRole="button" style={styles.cancel}>
        <Text style={styles.cancelText}>{t('choice.cancel')}</Text>
      </Pressable>
    </Sheet>
  );
}

function choiceStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    scrim: { flex: 1, backgroundColor: 'rgba(0,0,0,0.55)', justifyContent: 'center', alignItems: 'center', padding: 16 },
    sheet: {
      width: '100%',
      maxWidth: 420,
      backgroundColor: colors.surface,
      borderColor: colors.accent,
      borderWidth: 1,
      borderRadius: 14,
      padding: 18,
      gap: 14,
    },
    title: { color: colors.text, fontSize: 18, fontWeight: '700', textAlign: 'center' },
    grid: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, justifyContent: 'center' },
    option: {
      minWidth: 150,
      flexGrow: 1,
      paddingVertical: 16,
      paddingHorizontal: 14,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.bg,
      alignItems: 'center',
    },
    suggested: { borderColor: colors.accent, borderWidth: 2 },
    pressed: { opacity: 0.8, transform: [{ scale: 0.97 }] },
    optionText: { color: colors.text, fontSize: 20, fontWeight: '700' },
    cancel: { alignSelf: 'center', paddingVertical: 8, paddingHorizontal: 16 },
    cancelText: { color: colors.muted, fontSize: 15 },
  });
}
