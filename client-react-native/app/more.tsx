import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * The two things a player wants only occasionally: a running score kept by
 * hand, and how everyone stacks up over time. Neither belongs on the main
 * menu — that screen answers "what do I do right now", and these answer "how
 * did it go" — so they live here instead, one tap further in.
 *
 * Both need an account, and a guest is not one: what these screens keep is
 * kept server-side against a player, and a guest has nowhere for it to be
 * kept. They are drawn disabled rather than hidden, because a player who
 * cannot find a feature concludes it does not exist, while a player who can
 * see it greyed out with the reason under it has been told what signing in
 * is *for*. The screens behind them say the same thing again — see
 * `SignInRequired`, since a gate that only exists on the button is not one.
 */
export default function MoreScreen() {
  const { session } = useSession();
  useLocale();

  const signedIn = !!session && !session.isGuest;

  return (
    <Screen title={t('nav.more')} scroll>
      <Entry
        testID="more-score-table"
        label={t('more.scoreTable')}
        enabled={signedIn}
        primary
        onPress={() => router.push('/scoring')}
      />
      <Entry
        testID="more-stats"
        label={t('more.stats')}
        enabled={signedIn}
        onPress={() => router.push('/stats')}
      />

      <Pressable style={{ marginTop: 16 }} onPress={() => router.back()}>
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}

function Entry({
  label,
  onPress,
  testID,
  enabled,
  primary,
}: {
  label: string;
  onPress: () => void;
  testID: string;
  enabled: boolean;
  primary?: boolean;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      // Both halves, because they answer different questions: the state is
      // what a screen reader announces, and the flag is what stops the tap.
      accessibilityState={{ disabled: !enabled }}
      accessibilityLabel={enabled ? label : `${label} (${t('more.needsAccount')})`}
      disabled={!enabled}
      onPress={onPress}
      // A disabled entry always wears the secondary look, never the primary
      // one dimmed: the primary fill is a pale blue carrying dark text, and
      // the grey of the reason underneath is unreadable on it. Dimmed, the
      // dark surface keeps both legible.
      style={[
        shared.button,
        (!primary || !enabled) && shared.buttonSecondary,
        !enabled && styles.disabled,
      ]}
    >
      <Text
        style={[shared.buttonText, (!primary || !enabled) && shared.buttonTextSecondary]}
      >
        {label}
      </Text>
      {!enabled ? (
        <Text style={styles.hint} testID={`${testID}-hint`}>
          ({t('more.needsAccount')})
        </Text>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  // The same fill at reduced opacity, which is the step the primary buttons
  // already use for disabled — see `accentButton` in the palette.
  disabled: { opacity: 0.4 },
  hint: { color: colors.muted, fontSize: 12, marginTop: 2 },
});
