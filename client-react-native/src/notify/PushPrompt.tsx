import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { useSession } from '@/src/context/SessionContext';
import { t } from '@/src/lib/i18n';
import type { SkinColors } from '@/src/skins/types';
import { colors } from '@/src/theme';
import { pushPromptSnoozed, snoozePushPrompt } from '@/src/notify/prefs';
import { enablePush, pushStatus } from '@/src/notify/push';

/**
 * The card that asks before the OS does.
 *
 * An OS permission prompt can be answered once; after a "Don't allow" the
 * app can only point at system settings. So the prompt is never shown at
 * launch, where it means nothing, and never by itself: this card comes first,
 * at the three moments the answer is obviously useful — a game against a
 * person just ended, somebody was just added to the circle, a table for
 * friends was just opened — and the OS prompt follows only a tap on Enable.
 *
 * "Later" keeps it away for a fortnight (`PUSH_LATER_MS`). It appears only
 * while the answer is still "never asked": a player who said yes or no to the
 * OS has nothing left to be asked here.
 */
export function PushPrompt({ palette = colors }: { palette?: SkinColors }) {
  const { onlineSession } = useSession();
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!onlineSession) {
      setVisible(false);
      return;
    }
    let live = true;
    void (async () => {
      const [status, snoozed] = await Promise.all([pushStatus(), pushPromptSnoozed()]);
      if (live) setVisible(status === 'default' && !snoozed);
    })();
    return () => {
      live = false;
    };
  }, [onlineSession]);

  if (!visible) return null;

  return (
    <View
      testID="push-prompt"
      style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.border }]}
    >
      <Text style={[styles.title, { color: palette.text }]}>{t('notify.prompt.title')}</Text>
      <Text style={[styles.body, { color: palette.muted }]}>{t('notify.prompt.body')}</Text>
      <View style={styles.actions}>
        <Pressable
          testID="push-prompt-enable"
          accessibilityRole="button"
          disabled={busy}
          onPress={async () => {
            setBusy(true);
            try {
              await enablePush();
            } finally {
              setBusy(false);
              // Whatever the OS was told, this card has nothing more to ask.
              setVisible(false);
            }
          }}
          style={[styles.enable, { backgroundColor: palette.accentButton }]}
        >
          <Text style={[styles.enableText, { color: palette.onAccent }]}>{t('notify.prompt.enable')}</Text>
        </Pressable>
        <Pressable
          testID="push-prompt-later"
          accessibilityRole="button"
          onPress={() => {
            setVisible(false);
            void snoozePushPrompt();
          }}
          style={styles.later}
        >
          <Text style={[styles.laterText, { color: palette.muted }]}>{t('notify.prompt.later')}</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: 12, padding: 14, marginTop: 12 },
  title: { fontSize: 15, fontWeight: '700' },
  body: { fontSize: 13, marginTop: 4 },
  actions: { flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 10 },
  enable: { paddingVertical: 9, paddingHorizontal: 18, borderRadius: 8 },
  enableText: { fontSize: 14, fontWeight: '700' },
  later: { paddingVertical: 9, paddingHorizontal: 8 },
  laterText: { fontSize: 14, fontWeight: '600' },
});
