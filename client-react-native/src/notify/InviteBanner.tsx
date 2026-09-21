import { useEffect, useRef, useState } from 'react';
import { Animated, Modal, Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Avatar } from '@/src/components/avatars/Avatar';
import { useLocale } from '@/src/hooks/useLocale';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { ARRIVAL_EASING, ms } from '@/src/lib/motion';
import type { SkinColors } from '@/src/skins/types';
import { colors } from '@/src/theme';
import { useInvites } from '@/src/notify/InviteProvider';
import { InviteRow, inviteAvatar, inviteDetail, inviteHeadline } from '@/src/notify/InviteRow';

/** How long the banner stays before it tucks itself away into the list. */
export const BANNER_MS = 20_000;

/**
 * The invite banner: over every screen, in the way of none.
 *
 * It slides down from the top with the newest invite, offers Join and Not
 * now, and after twenty seconds goes away on its own — without dropping the
 * invite, which stays on the home screen's list. It covers the top of the
 * screen and nothing else, and nothing under it is blocked: the wrapper lets
 * every touch outside the card through.
 *
 * At a table it gives way altogether to a pill ("1 table waiting"). A banner
 * sliding over somebody's hand mid-turn is how a misplaced discard happens,
 * and a player in a game has already chosen what they are doing; the pill
 * says there is something else when they want it.
 *
 * Coloured by the skin at a table and by the app's palette everywhere else,
 * and only coloured: the skin rule (src/skins/types.ts) holds here as on the
 * board, so no size or testID differs between looks.
 */
export function InviteBanner() {
  useLocale();
  const { banner, invites, inMatch, join, dismiss, muteHost, shelve, joiningId, error, clearError } =
    useInvites();
  const skin = useSkin();
  const palette: SkinColors = inMatch ? skin.colors : colors;
  const insets = useSafeAreaInsets();
  const stillness = useReducedMotion();
  const [menuOpen, setMenuOpen] = useState(false);
  const [listOpen, setListOpen] = useState(false);

  const shown = inMatch ? null : banner;
  const shownId = shown?.id ?? '';

  // The banner's own clock: twenty seconds, then into the list. Restarted
  // for each new invite, and held while the player has the menu open — a
  // banner that vanished under a thumb reaching for "mute" would be rude.
  useEffect(() => {
    if (!shownId || menuOpen) return undefined;
    const timer = setTimeout(() => shelve(shownId), BANNER_MS);
    return () => clearTimeout(timer);
  }, [shownId, menuOpen, shelve]);

  useEffect(() => {
    setMenuOpen(false);
  }, [shownId]);

  // An error is a moment, like the invite that caused it.
  useEffect(() => {
    if (!error) return undefined;
    const timer = setTimeout(clearError, 6000);
    return () => clearTimeout(timer);
  }, [error, clearError]);

  const arrival = useRef(new Animated.Value(1)).current;
  useEffect(() => {
    if (!shownId) return;
    if (stillness) {
      arrival.setValue(1);
      return;
    }
    arrival.setValue(0);
    Animated.timing(arrival, {
      toValue: 1,
      duration: ms(220),
      easing: ARRIVAL_EASING,
      useNativeDriver: true,
    }).start();
  }, [shownId, stillness, arrival]);

  const others = invites.length - (shown ? 1 : 0);

  const list = (
    <Modal transparent animationType={stillness ? 'none' : 'fade'} visible={listOpen} onRequestClose={() => setListOpen(false)}>
      <Pressable style={styles.backdrop} onPress={() => setListOpen(false)} testID="invite-list-backdrop">
        <Pressable
          style={[styles.sheet, { backgroundColor: palette.surface, borderColor: palette.border, marginTop: insets.top + 48 }]}
          onPress={() => {}}
          testID="invite-list"
        >
          <Text style={[styles.sheetTitle, { color: palette.text }]}>{t('notify.waitingCard.title')}</Text>
          {invites.length === 0 ? (
            <Text style={[styles.detail, { color: palette.muted }]}>{t('notify.waitingCard.empty')}</Text>
          ) : (
            invites.map((i) => (
              <InviteRow
                key={i.id}
                invite={i}
                palette={palette}
                busy={joiningId === i.id}
                onJoin={() => {
                  setListOpen(false);
                  void join(i.id);
                }}
                onDismiss={() => dismiss(i.id)}
              />
            ))
          )}
        </Pressable>
      </Pressable>
    </Modal>
  );

  if (inMatch) {
    if (invites.length === 0 && !listOpen) return null;
    return (
      <View pointerEvents="box-none" style={[styles.layer, { top: insets.top + 6 }]}>
        {invites.length > 0 ? (
          <Pressable
            testID="invite-pill"
            accessibilityRole="button"
            onPress={() => setListOpen(true)}
            style={[styles.pill, { backgroundColor: palette.surface, borderColor: palette.accent }]}
          >
            <Text style={[styles.pillText, { color: palette.text }]}>
              {invites.length === 1 ? t('notify.pillOne') : t('notify.pillMany', { n: invites.length })}
            </Text>
          </Pressable>
        ) : null}
        {list}
      </View>
    );
  }

  if (!shown) {
    return error ? (
      <View pointerEvents="box-none" style={[styles.layer, { top: insets.top + 6 }]}>
        <View style={[styles.card, { backgroundColor: palette.surface, borderColor: palette.danger }]}>
          <Text style={[styles.error, { color: palette.danger }]} testID="invite-banner-error">
            {error}
          </Text>
        </View>
        {list}
      </View>
    ) : (
      list
    );
  }

  const muteable = !!shown.host.key;

  return (
    <View pointerEvents="box-none" style={[styles.layer, { top: insets.top + 6 }]}>
      <Animated.View
        testID="invite-banner"
        accessibilityRole="alert"
        style={[
          styles.card,
          { backgroundColor: palette.surface, borderColor: palette.accent },
          {
            opacity: arrival,
            transform: [{ translateY: arrival.interpolate({ inputRange: [0, 1], outputRange: [-24, 0] }) }],
          },
        ]}
      >
        <View style={styles.top}>
          <Avatar spec={inviteAvatar(shown)} size={32} />
          <View style={styles.text}>
            <Text testID="invite-banner-text" style={[styles.headline, { color: palette.text }]} numberOfLines={2}>
              {inviteHeadline(shown)}
            </Text>
            <Text style={[styles.detail, { color: palette.muted }]} numberOfLines={1}>
              {inviteDetail(shown)}
            </Text>
          </View>
          {muteable ? (
            <Pressable
              testID="invite-banner-menu"
              accessibilityRole="button"
              accessibilityLabel={t('notify.moreOptions')}
              onPress={() => setMenuOpen((o) => !o)}
              style={styles.menuButton}
            >
              <Text style={[styles.menuGlyph, { color: palette.muted }]}>⋯</Text>
            </Pressable>
          ) : null}
        </View>

        {menuOpen ? (
          <Pressable
            testID="invite-banner-mute"
            accessibilityRole="button"
            onPress={() => void muteHost(shown.id)}
            style={[styles.muteRow, { borderColor: palette.border }]}
          >
            <Text style={[styles.muteText, { color: palette.danger }]}>
              {t('notify.muteHost', { host: shown.host.name || t('notify.someone') })}
            </Text>
          </Pressable>
        ) : null}

        {error ? (
          <Text style={[styles.error, { color: palette.danger }]} testID="invite-banner-error">
            {error}
          </Text>
        ) : null}

        <View style={styles.actions}>
          <Pressable
            testID="invite-banner-join"
            accessibilityRole="button"
            disabled={joiningId === shown.id}
            onPress={() => void join(shown.id)}
            style={[styles.join, { backgroundColor: palette.accentButton }]}
          >
            <Text style={[styles.joinText, { color: palette.onAccent }]}>
              {joiningId === shown.id
                ? t('notify.joining')
                : shown.source === 'waiting-room'
                  ? t('notify.goToTable')
                  : t('notify.join')}
            </Text>
          </Pressable>
          <Pressable
            testID="invite-banner-dismiss"
            accessibilityRole="button"
            onPress={() => dismiss(shown.id)}
            style={[styles.notNow, { borderColor: palette.border }]}
          >
            <Text style={[styles.notNowText, { color: palette.text }]}>{t('notify.notNow')}</Text>
          </Pressable>
          {others > 0 ? (
            <Pressable
              testID="invite-banner-more"
              accessibilityRole="button"
              onPress={() => setListOpen(true)}
              style={styles.more}
            >
              <Text style={[styles.moreText, { color: palette.accent }]}>
                {t('notify.banner.more', { n: others })}
              </Text>
            </Pressable>
          ) : null}
        </View>
      </Animated.View>
      {list}
    </View>
  );
}

const styles = StyleSheet.create({
  // Wide as the screen, but only the card inside catches touches.
  layer: { position: 'absolute', left: 8, right: 8, zIndex: 1000, elevation: 1000, alignItems: 'center' },
  card: {
    width: '100%',
    maxWidth: 480,
    borderWidth: 1,
    borderRadius: 12,
    padding: 12,
    shadowColor: '#000',
    shadowOpacity: 0.35,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 8,
  },
  top: { flexDirection: 'row', alignItems: 'center', gap: 10 },
  text: { flex: 1, minWidth: 0 },
  headline: { fontSize: 15, fontWeight: '700' },
  detail: { fontSize: 12, marginTop: 2 },
  menuButton: { paddingHorizontal: 8, paddingVertical: 4 },
  menuGlyph: { fontSize: 20, fontWeight: '700' },
  muteRow: { marginTop: 10, paddingTop: 10, borderTopWidth: 1 },
  muteText: { fontSize: 14, fontWeight: '600' },
  error: { fontSize: 13, marginTop: 8 },
  actions: { flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 10 },
  join: { paddingVertical: 10, paddingHorizontal: 18, borderRadius: 8 },
  joinText: { fontSize: 15, fontWeight: '700' },
  notNow: { paddingVertical: 9, paddingHorizontal: 14, borderRadius: 8, borderWidth: 1 },
  notNowText: { fontSize: 14, fontWeight: '600' },
  more: { marginLeft: 'auto', paddingVertical: 8, paddingHorizontal: 4 },
  moreText: { fontSize: 13, fontWeight: '600' },
  pill: { borderWidth: 1, borderRadius: 999, paddingVertical: 5, paddingHorizontal: 12 },
  pillText: { fontSize: 12, fontWeight: '700' },
  backdrop: { flex: 1, backgroundColor: 'rgba(0,0,0,0.5)', alignItems: 'center', paddingHorizontal: 12 },
  sheet: { width: '100%', maxWidth: 480, borderWidth: 1, borderRadius: 12, padding: 14 },
  sheetTitle: { fontSize: 15, fontWeight: '700', marginBottom: 6 },
});
