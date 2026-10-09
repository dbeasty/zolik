import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * Getting up from the table, asked once. The one control at a table that
 * cannot be taken back — the seat is gone for the rest of the match — so it
 * is the one that asks first.
 */
export function LeaveSheet({ visible, onLeave, onCancel }: { visible: boolean; onLeave: () => void; onCancel: () => void }) {
  return (
    <Modal transparent animationType="fade" visible={visible} onRequestClose={onCancel}>
      <Pressable style={styles.backdrop} onPress={onCancel} testID="leave-backdrop">
        {/* Stops a press inside the panel from closing it. */}
        <Pressable style={styles.sheet} onPress={() => {}} testID="leave-sheet">
          <Text style={styles.title}>{t('leave.confirmTitle')}</Text>
          <Text style={styles.body}>{t('leave.confirmBody')}</Text>
          <View style={styles.row}>
            <Pressable testID="leave-cancel" onPress={onCancel} style={styles.pill} accessibilityRole="button">
              <Text style={styles.pillText}>{t('leave.cancel')}</Text>
            </Pressable>
            <Pressable testID="leave-confirm" onPress={onLeave} style={[styles.pill, styles.danger]} accessibilityRole="button">
              <Text style={styles.pillText}>{t('leave.confirm')}</Text>
            </Pressable>
          </View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
  },
  sheet: {
    minWidth: 260,
    maxWidth: 360,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    padding: 16,
    gap: 12,
  },
  title: { color: colors.text, fontWeight: '700', fontSize: 15 },
  body: { color: colors.muted, fontSize: 14 },
  row: { flexDirection: 'row', justifyContent: 'flex-end', gap: 8 },
  pill: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
  },
  danger: { borderColor: colors.danger },
  pillText: { color: colors.text, fontWeight: '600' },
});
