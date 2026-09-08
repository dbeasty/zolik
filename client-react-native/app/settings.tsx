import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AvatarPicker } from '@/src/components/avatars/AvatarPicker';
import { LanguagePicker } from '@/src/components/LanguagePicker';
import { LegalLinks } from '@/src/components/LegalLinks';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useAvatarControls } from '@/src/hooks/useAvatar';
import { useLocale } from '@/src/hooks/useLocale';
import { useSkinControls } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * The things a player chooses about themselves and about the board.
 *
 * There was nowhere to put either of these before: the look was cycled by a
 * button on the match screen, and a face could not be chosen at all. They are
 * together because they are the same kind of decision — how this looks — and
 * because one screen that answers "where do I change that?" beats two places
 * that each answer half of it.
 *
 * A signed-in player's face is written to their account as well as this
 * device, so it follows them; a guest's lives on the device alone, which is
 * the most an identity with nowhere to be stored can manage. `useAvatar`
 * decides which, so nothing here has to.
 *
 * Language joined them for the same reason, and is the one setting on this
 * screen a player may arrive at unable to read the rest of.
 */
export default function SettingsScreen() {
  const { session } = useSession();
  // Not for a value this screen uses — for the subscription. `t` reads a
  // module global, so without this the screen would keep its old words after
  // the picker below changed the language.
  useLocale();
  const { avatarId, setAvatarId } = useAvatarControls();
  const { skin, skins, setSkinId } = useSkinControls();

  const signedIn = !!session && !session.isGuest;

  return (
    <Screen title={t('settings.title')} subtitle={t('settings.subtitle')} scroll>
      <Text style={shared.status}>
        {!session
          ? t('settings.notSignedIn')
          : signedIn
            ? t('settings.signedInAs', { username: session.username })
            : t('settings.playingAsGuest', { username: session.username })}
      </Text>

      <View style={shared.card}>
        <Text style={styles.heading}>{t('settings.face.heading')}</Text>
        <Text style={shared.status}>
          {signedIn ? t('settings.face.account') : t('settings.face.device')}
        </Text>
        <AvatarPicker value={avatarId} onChange={setAvatarId} />
      </View>

      <View style={shared.card}>
        <Text style={styles.heading}>{t('settings.skin.heading')}</Text>
        <View style={styles.skins}>
          {skins.map((s) => {
            const picked = s.id === skin.id;
            return (
              <Pressable
                key={s.id}
                testID={`skin-choice-${s.id}`}
                accessibilityRole="radio"
                accessibilityState={{ checked: picked }}
                onPress={() => setSkinId(s.id)}
                style={[styles.skin, picked && styles.skinPicked]}
              >
                {/* A stripe of the felt itself — a look is easier to pick by
                    looking at than by reading its name. The middle stop, not
                    the first: a felt's gradient runs dark at the edges, and
                    the darkest end of it says nothing about the colour. */}
                <View
                  style={[
                    styles.swatch,
                    { backgroundColor: s.table.background[Math.floor(s.table.background.length / 2)] },
                  ]}
                >
                  <View style={[styles.swatchTrim, { backgroundColor: s.colors.gold }]} />
                </View>
                <Text style={[styles.skinName, picked && styles.skinNamePicked]}>{s.label}</Text>
              </Pressable>
            );
          })}
        </View>
      </View>

      <View style={shared.card}>
        <Text style={styles.heading}>{t('settings.language.heading')}</Text>
        <Text style={shared.status}>{t('settings.language.status')}</Text>
        <LanguagePicker />
      </View>

      {/* Where a player goes looking for the notices once the sign-in screen
          that first showed them is behind them. */}
      <View style={shared.card}>
        <Text style={styles.heading}>{t('settings.legal.heading')}</Text>
        <Text style={shared.status}>{t('settings.legal.status')}</Text>
        <LegalLinks style={{ marginTop: 10 }} />
      </View>

      {!signedIn ? (
        <Pressable style={shared.button} onPress={() => router.push('/auth/login')}>
          <Text style={shared.buttonText}>{t('settings.signIn')}</Text>
        </Pressable>
      ) : null}
      <Pressable onPress={() => router.back()}>
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
  skins: { flexDirection: 'row', gap: 12, marginTop: 8 },
  // Two pixels of border always, transparent when unpicked — picking one must
  // not move the one beside it.
  skin: { alignItems: 'center', gap: 4, borderWidth: 2, borderColor: 'transparent', borderRadius: 10, padding: 6 },
  skinPicked: { borderColor: colors.gold },
  swatch: { width: 64, height: 40, borderRadius: 6, overflow: 'hidden', justifyContent: 'flex-end' },
  swatchTrim: { height: 4 },
  skinName: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  skinNamePicked: { color: colors.gold },
});
