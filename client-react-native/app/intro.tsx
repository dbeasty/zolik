import { router, type Href } from 'expo-router';
import { Platform, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MatchModule } from '@/src/api/matchTypes';
import { heading } from '@/src/a11y/props';
import { GameButtons } from '@/src/components/GameButtons';
import { useSession } from '@/src/context/SessionContext';
import { useMetrics } from '@/src/hooks/useMetrics';
import { markIntroSeen } from '@/src/lib/introStore';
import { savePendingDestination } from '@/src/lib/pendingDestination';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * Three short reasons to tap Play, each led by a card suit rather than an
 * icon glyph — the app draws its own cards and avatars but has no icon font
 * (see `InviteRow`'s `✕` for the house style: plain text/unicode accents,
 * not a library), and a suit mark is free thematic decoration on a screen
 * about a card-game server.
 */
const BULLETS: { mark: string; key: string }[] = [
  { mark: '♠', key: 'intro.bulletFriends' },
  { mark: '♥', key: 'intro.bulletCrossDevice' },
  { mark: '♦', key: 'intro.bulletGuest' },
];

/**
 * The first-run screen — what the app is, before anyone is asked to sign in
 * or pick a game. Shown once per device; see `src/lib/introStore.ts`.
 *
 * Edge-to-edge and header-less on purpose (`headerShown: false` in
 * `app/_layout.tsx`): this is a one-time marketing moment, not a utility
 * screen, so it does not reuse `<Screen>`'s title-bar layout.
 *
 * One layout, two arrangements, same as the board itself splits on
 * `useMetrics().narrow` — stacked and full-width under 768px, capped and
 * centered above it. See `app/match/[matchId].tsx` for the same idiom.
 */
export default function IntroScreen() {
  const { narrow } = useMetrics();
  const { session } = useSession();

  const onPlay = () => {
    // Fire-and-forget: a slow or failed write should never hold up the tap
    // that is the entire point of this screen. Worst case, storage failed
    // and the intro shows again next time — see `introStore.ts`.
    void markIntroSeen();
    router.replace('/');
  };

  // A game button goes straight to that game's setup, and "Join a table"
  // straight to the code entry. Without a session the guest screen comes
  // first, and the destination is where it lands afterwards — the same
  // handoff a shared link uses (see `pendingDestination.ts`).
  const goTo = async (path: string) => {
    void markIntroSeen();
    if (!session) {
      await savePendingDestination(path);
      router.replace('/auth/guest');
      return;
    }
    router.replace(path as Href);
  };
  const onPickGame = (mod: MatchModule) =>
    goTo(`/lobby/games?moduleId=${encodeURIComponent(mod.id)}`);

  return (
    <SafeAreaView style={styles.safe} edges={['top', 'bottom', 'left', 'right']}>
      {/* Scrolls once it no longer fits — at 200% text, or a short window —
          rather than cutting the Play button off below the fold. `flexGrow`
          keeps the spacer pushing Play to the bottom while it does fit. */}
      <ScrollView
        contentContainerStyle={styles.scroll}
        {...(Platform.OS === 'web' ? { tabIndex: 0 as const } : {})}
      >
        <View style={[styles.content, !narrow && styles.contentWide]}>
          <View style={styles.hero}>
            {/* The page's only heading: this screen has no navigation bar to
                carry one. */}
            <Text style={styles.wordmark} {...heading(1)}>
              Jokerless
            </Text>
            <Text style={styles.tagline}>{t('intro.tagline')}</Text>
          </View>

          <View style={styles.games}>
            <GameButtons onPick={(mod) => void onPickGame(mod)} />
            {/* For a code heard out loud rather than a link followed: without
                this, the way to the join screen ran through the main menu. */}
            <Pressable
              testID="intro-join"
              accessibilityRole="link"
              onPress={() => void goTo('/lobby/join')}
              style={({ pressed }) => [styles.joinLink, pressed && styles.joinLinkPressed]}
            >
              <Text style={styles.joinLinkText}>{t('nav.join')} ›</Text>
            </Pressable>
          </View>

          <View style={[styles.bullets, !narrow && styles.bulletsRow]}>
            {BULLETS.map((b) => (
              <View key={b.key} style={[styles.bullet, !narrow && styles.bulletColumn]}>
                <Text style={styles.bulletMark} aria-hidden>
                  {b.mark}
                </Text>
                <Text style={styles.bulletText}>{t(b.key)}</Text>
              </View>
            ))}
          </View>

          {narrow ? <View style={styles.spacer} /> : null}

          <Pressable
            role="button"
            testID="intro-play"
            style={[styles.playButton, !narrow && styles.playButtonWide]}
            onPress={onPlay}
          >
            <Text style={styles.playButtonText}>{t('home.play')}</Text>
          </Pressable>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {
    flex: 1,
    backgroundColor: colors.bg,
  },
  scroll: {
    flexGrow: 1,
  },
  content: {
    flex: 1,
    padding: 24,
  },
  // Wide/web: the column is capped and centered rather than stretched edge
  // to edge, matching the board's own `maxWidth` + `alignSelf: 'center'`
  // convention (see `app/match/[matchId].tsx`).
  contentWide: {
    width: '100%',
    maxWidth: 640,
    alignSelf: 'center',
    justifyContent: 'center',
  },
  hero: {
    alignItems: 'center',
    marginTop: 24,
  },
  wordmark: {
    fontSize: 34,
    fontWeight: '700',
    color: colors.text,
    letterSpacing: -0.5,
  },
  tagline: {
    fontSize: 15,
    color: colors.muted,
    marginTop: 10,
    textAlign: 'center',
  },
  games: {
    marginTop: 28,
  },
  joinLink: {
    alignSelf: 'center',
    marginTop: 14,
    paddingVertical: 6,
    paddingHorizontal: 12,
    borderRadius: 8,
  },
  joinLinkPressed: {
    backgroundColor: colors.surface,
  },
  joinLinkText: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.accent,
  },
  bullets: {
    marginTop: 28,
    gap: 14,
  },
  bulletsRow: {
    flexDirection: 'row',
    gap: 20,
  },
  bullet: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 10,
  },
  // Wide: each bullet becomes its own column, mark above text, matching the
  // three-column arrangement in the web mockup.
  bulletColumn: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'flex-start',
    gap: 6,
  },
  bulletMark: {
    fontSize: 18,
    color: colors.accent,
    lineHeight: 20,
  },
  bulletText: {
    fontSize: 13,
    color: colors.text,
    lineHeight: 19,
    flexShrink: 1,
  },
  spacer: {
    flex: 1,
  },
  playButton: {
    backgroundColor: colors.accentButton,
    paddingVertical: 15,
    borderRadius: 10,
    alignItems: 'center',
    marginTop: 28,
  },
  // Wide: a landing-page CTA sized to its label, centered, not full-bleed.
  playButtonWide: {
    alignSelf: 'center',
    paddingHorizontal: 64,
  },
  playButtonText: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.onAccent,
  },
});
