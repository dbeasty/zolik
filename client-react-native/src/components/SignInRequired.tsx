import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * What a screen shows instead of itself when it needs an account and there
 * isn't one.
 *
 * The menu already draws these entries disabled, so nobody arrives here by
 * tapping. They arrive by typing the address, or by following a link, or by
 * signing out on a device that had the screen open — and a gate that only
 * exists on the button in front of it is not a gate. Stated once here rather
 * than twice in the screens, so the two cannot come to disagree about who is
 * allowed in or why.
 *
 * A guest counts as no account on purpose: what is kept here is kept *with*
 * an account, and a guest has nowhere for it to be kept.
 */
export function SignInRequired({ title }: { title: string }) {
  useLocale();

  return (
    <Screen title={title}>
      <View style={shared.card} testID="sign-in-required">
        <Text style={styles.heading}>{t('gate.title')}</Text>
        <Text style={shared.status}>{t('gate.body')}</Text>
      </View>
      <Pressable
        style={shared.button}
        testID="sign-in-required-signin"
        onPress={() => router.push('/auth/login')}
      >
        <Text style={shared.buttonText}>{t('settings.signIn')}</Text>
      </Pressable>
      <Pressable onPress={() => router.back()}>
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
});
