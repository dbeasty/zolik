import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useMetrics } from '@/src/hooks/useMetrics';
import { markIntroSeen } from '@/src/lib/introStore';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * The four games this build hosts, as shown on the intro screen.
 *
 * Plain text, not `moduleName()`/`t()` — game names are one of the things
 * `src/lib/gameLabels.ts` deliberately keeps out of the locale files
 * ("Texas Hold'em" reads the same in every language), and this screen has
 * nothing to look them up against anyway: it renders before any session or
 * `/modules` fetch, for a visitor who has neither yet.
 */
const GAMES = ['Žolíky', 'Prší', 'Canasta', "Hold'em"];

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

  const onPlay = () => {
    // Fire-and-forget: a slow or failed write should never hold up the tap
    // that is the entire point of this screen. Worst case, storage failed
    // and the intro shows again next time — see `introStore.ts`.
    void markIntroSeen();
    router.replace('/');
  };

  return (
    <SafeAreaView style={styles.safe} edges={['top', 'bottom', 'left', 'right']}>
      <View style={[styles.content, !narrow && styles.contentWide]}>
        <View style={styles.hero}>
          <Text style={styles.wordmark}>Jokerless</Text>
          <Text style={styles.tagline}>{t('intro.tagline')}</Text>
        </View>

        <View style={styles.games}>
          {GAMES.map((name) => (
            <View key={name} style={styles.gameChip}>
              <Text style={styles.gameChipText}>{name}</Text>
            </View>
          ))}
        </View>

        <View style={[styles.bullets, !narrow && styles.bulletsRow]}>
          {BULLETS.map((b) => (
            <View key={b.key} style={[styles.bullet, !narrow && styles.bulletColumn]}>
              <Text style={styles.bulletMark}>{b.mark}</Text>
              <Text style={styles.bulletText}>{t(b.key)}</Text>
            </View>
          ))}
        </View>

        {narrow ? <View style={styles.spacer} /> : null}

        <Pressable
          testID="intro-play"
          style={[styles.playButton, !narrow && styles.playButtonWide]}
          onPress={onPlay}
        >
          <Text style={styles.playButtonText}>{t('home.play')}</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {
    flex: 1,
    backgroundColor: colors.bg,
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
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginTop: 28,
  },
  gameChip: {
    flexGrow: 1,
    flexBasis: '45%',
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 12,
    alignItems: 'center',
  },
  gameChipText: {
    fontSize: 13,
    color: colors.text,
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
