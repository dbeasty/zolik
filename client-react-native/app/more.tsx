import { router } from 'expo-router';
import { Pressable, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import { shared } from '@/src/theme';

/**
 * The two things a player wants only occasionally: a running score kept by
 * hand for a game away from the server, and how everyone stacks up over
 * time. Neither belongs on the main menu — that screen answers "what do I do
 * right now", and these answer "how did it go" — so they live here instead,
 * one tap further in.
 */
export default function MoreScreen() {
  useLocale();

  return (
    <Screen title={t('nav.more')} scroll>
      <Pressable
        testID="more-score-table"
        style={shared.button}
        onPress={() => router.push('/scoring')}
      >
        <Text style={shared.buttonText}>{t('more.scoreTable')}</Text>
      </Pressable>
      <Pressable
        testID="more-stats"
        style={[shared.button, shared.buttonSecondary]}
        onPress={() => router.push('/stats')}
      >
        <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('more.stats')}</Text>
      </Pressable>

      <Pressable style={{ marginTop: 16 }} onPress={() => router.back()}>
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}
