import { useMemo } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { Sheet } from '@/src/a11y/Sheet';
import type { Fact, MatchPlayer } from '@/src/api/matchTypes';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

/**
 * What the table still requires of this viewer — the first meld's size, the
 * canastas before going out — in a sheet of its own rather than the log box.
 *
 * The module sends the hints most relevant first and only while unmet, so
 * this lists them in the order given and a met requirement is simply gone.
 * It knows nothing of any game.
 */
export function HintsSheet({
  hints,
  players,
  onClose,
}: {
  hints: Fact[];
  players: MatchPlayer[];
  onClose: () => void;
}) {
  const skin = useSkin();
  const styles = useMemo(() => hintsStyles(skin), [skin]);
  return (
    <Sheet
      visible
      onClose={onClose}
      label={t('hints.title')}
      backdropStyle={styles.backdrop}
      backdropTestID="hints-sheet-backdrop"
      style={styles.sheet}
      testID="hints-sheet"
    >
      <Text style={styles.title}>{t('hints.title')}</Text>
      <ScrollView contentContainerStyle={styles.body}>
        {hints.map((f, i) => (
          <Text key={i} testID={`hint-${i}`} style={[styles.hint, i === 0 && styles.first]}>
            {factText(f, players)}
          </Text>
        ))}
      </ScrollView>
      <Pressable style={styles.close} onPress={onClose} testID="hints-close" accessibilityRole="button">
        <Text style={styles.closeText}>{t('why.close')}</Text>
      </Pressable>
    </Sheet>
  );
}

function hintsStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    backdrop: { flex: 1, backgroundColor: 'rgba(0,0,0,0.6)', justifyContent: 'flex-end' },
    sheet: {
      backgroundColor: colors.surface,
      borderTopWidth: 1,
      borderLeftWidth: 1,
      borderRightWidth: 1,
      borderColor: colors.border,
      borderTopLeftRadius: 14,
      borderTopRightRadius: 14,
      paddingHorizontal: 16,
      paddingTop: 16,
      paddingBottom: 20,
      maxHeight: '70%',
      gap: 12,
    },
    title: { color: colors.muted, fontSize: 11, letterSpacing: 1, textTransform: 'uppercase', fontWeight: '600' },
    body: { gap: 10 },
    hint: { color: colors.text, fontSize: 15 },
    first: { color: colors.gold, fontWeight: '700', fontSize: 16 },
    close: { alignSelf: 'flex-end', minHeight: 36, paddingHorizontal: 14, justifyContent: 'center' },
    closeText: { color: colors.accent, fontSize: 15, fontWeight: '700' },
  });
}
