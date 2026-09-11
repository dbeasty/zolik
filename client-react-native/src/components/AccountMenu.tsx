import { router } from 'expo-router';
import { useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { useSession } from '@/src/context/SessionContext';
import { useAvatarId } from '@/src/hooks/useAvatar';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * Who you are, and everything that is about you rather than about playing.
 *
 * The main menu used to carry all of this as buttons of its own — settings,
 * signing out, the second-tier screens — which put "Sign out" the same size
 * and the same distance from the thumb as "Play". A menu whose every item
 * looks equally likely is a menu that has not been designed; these are the
 * items a player touches once a session, so they fold behind one face in the
 * corner and leave the screen below saying what it is for.
 *
 * The face is the button on purpose. It is the one control that can answer
 * "am I signed in, and as whom" *without being opened* — the dot on its
 * corner is that answer at a glance, and the panel spells it out for anyone
 * the colour alone does not reach.
 */
export function AccountMenu() {
  const { session, logout } = useSession();
  const avatarId = useAvatarId();
  // `t` reads a module global, so without this the menu keeps the words it
  // first rendered with after the picker changes the language.
  useLocale();
  const [open, setOpen] = useState(false);

  const signedIn = !!session && !session.isGuest;
  // A guest's face is still their face: `avatarFor` derives a stable one from
  // the id when nothing has been chosen, so the corner is never empty.
  const spec = avatarFor(session?.userId ?? 'anon', false, avatarId ?? undefined);
  const statusColor = !session ? colors.muted : signedIn ? colors.success : colors.gold;
  const statusLabel = !session
    ? t('menu.notSignedIn')
    : signedIn
      ? t('menu.signedIn')
      : t('nav.guest');

  // The router's own route type, so a typo in a path below is a build error
  // rather than a dead menu item.
  const go = (path: Parameters<typeof router.push>[0]) => {
    setOpen(false);
    router.push(path);
  };

  return (
    <>
      <Pressable
        testID="account-menu-button"
        accessibilityRole="button"
        accessibilityLabel={t('menu.label')}
        onPress={() => setOpen(true)}
        style={styles.trigger}
      >
        <Avatar spec={spec} size={30} />
        {/* Ringed in the header's own colour, so it reads as a badge stuck on
            the face rather than as part of the drawing. */}
        <View style={[styles.dot, { backgroundColor: statusColor }]} />
      </Pressable>

      <Modal
        transparent
        animationType="fade"
        visible={open}
        onRequestClose={() => setOpen(false)}
      >
        <Pressable
          style={styles.backdrop}
          onPress={() => setOpen(false)}
          testID="account-menu-backdrop"
        >
          {/* Stops a press inside the panel from closing it. */}
          <Pressable style={styles.sheet} onPress={() => {}} testID="account-menu">
            <View style={styles.who}>
              <Avatar spec={spec} size={40} />
              <View style={styles.whoText}>
                <Text style={styles.name} numberOfLines={1}>
                  {session ? session.username : t('menu.notSignedIn')}
                </Text>
                {session ? (
                  <Text style={[styles.status, { color: statusColor }]} testID="account-menu-status">
                    {statusLabel}
                  </Text>
                ) : null}
              </View>
            </View>

            <View style={styles.rule} />

            <MenuItem
              label={t('nav.more')}
              testID="account-menu-more"
              onPress={() => go('/more')}
            />
            <MenuItem
              label={t('settings.title')}
              testID="account-menu-settings"
              onPress={() => go('/settings')}
            />
            {/* Above the sign-in items rather than below them, because the
                items below are about your account and this one is about the
                app — and because "About" under "Sign out" reads as part of
                leaving. */}
            <MenuItem
              label={t('nav.about')}
              testID="account-menu-about"
              onPress={() => go('/about')}
            />

            {/* Both sign-in items say the same two words, because that is what
                the action is. A guest gets the reason underneath in smaller
                type — the stats they are already building are the thing they
                stand to lose by staying anonymous — rather than folded into
                the label, where it made the item read as a different and
                longer errand than "sign in". */}
            {!session ? (
              <MenuItem
                label={t('settings.signIn')}
                testID="account-menu-signin"
                onPress={() => go('/auth/login')}
              />
            ) : signedIn ? (
              <MenuItem
                label={t('nav.account')}
                testID="account-menu-account"
                onPress={() => go('/account')}
              />
            ) : (
              <MenuItem
                label={t('settings.signIn')}
                hint={t('menu.keepStats')}
                testID="account-menu-signin"
                onPress={() => go('/auth/login')}
              />
            )}

            {session ? (
              <MenuItem
                label={t('menu.signOut')}
                testID="account-menu-signout"
                danger
                onPress={() => {
                  setOpen(false);
                  void logout();
                }}
              />
            ) : null}
          </Pressable>
        </Pressable>
      </Modal>
    </>
  );
}

function MenuItem({
  label,
  onPress,
  testID,
  danger,
  hint,
}: {
  label: string;
  onPress: () => void;
  testID: string;
  danger?: boolean;
  /** Why you would, under what it is. Bracketed and quieter than the label,
   *  so the item is still read as its verb. */
  hint?: string;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      // Read as one item by a screen reader, which has no smaller type to
      // hear the difference in.
      accessibilityLabel={hint ? `${label} (${hint})` : label}
      onPress={onPress}
      style={({ pressed }) => [styles.item, pressed && styles.itemPressed]}
    >
      <Text style={[styles.itemText, danger && styles.itemTextDanger]}>{label}</Text>
      {hint ? <Text style={styles.itemHint}>({hint})</Text> : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  trigger: { marginRight: 12, padding: 2 },
  dot: {
    position: 'absolute',
    right: 0,
    bottom: 0,
    width: 11,
    height: 11,
    borderRadius: 6,
    borderWidth: 2,
    borderColor: colors.surface,
  },
  // Anchored under the face that opened it rather than centred: a menu that
  // opens away from its own button reads as a different thing arriving.
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    alignItems: 'flex-end',
    justifyContent: 'flex-start',
    paddingTop: 56,
    paddingRight: 8,
  },
  sheet: {
    minWidth: 240,
    maxWidth: 320,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingVertical: 6,
  },
  who: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 10 },
  whoText: { flexShrink: 1 },
  name: { color: colors.text, fontSize: 15, fontWeight: '700' },
  status: { fontSize: 12, fontWeight: '600', marginTop: 2 },
  rule: { height: 1, backgroundColor: colors.border, marginBottom: 4 },
  item: { paddingHorizontal: 14, paddingVertical: 12 },
  itemPressed: { backgroundColor: colors.bg },
  itemText: { color: colors.text, fontSize: 15 },
  itemTextDanger: { color: colors.danger },
  itemHint: { color: colors.muted, fontSize: 12, marginTop: 2 },
});
