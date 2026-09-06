import { useEffect, useMemo, useRef, useState } from 'react';
import { AccessibilityInfo, Animated, StyleSheet, Text, View } from 'react-native';

import type { Fact, MatchPlayer, RoundLog, Standing } from '@/src/api/matchTypes';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';
import type { FlashKind } from '@/src/hooks/useResultsFlash';
import { t } from '@/src/lib/i18n';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import { ARRIVAL_EASING, ms } from '@/src/lib/motion';
import type { Skin } from '@/src/skins/types';

/**
 * The two seconds in which the table says a thing has ended.
 *
 * A round settling used to be reported by a panel quietly appearing above the
 * seat strip, the whole control set collapsing to one button below the hand, and
 * a grey sentence at the very bottom of the page saying what actually happened —
 * three answers in three places, none of them where a player is looking. This
 * announces the ending once, in the middle of the screen, and then gets out of
 * the way.
 *
 * Three things it deliberately is not:
 *
 *  - **Not a dialog.** Nothing to press, nothing to dismiss. It is
 *    `pointerEvents="none"` throughout, so a tap during those two seconds lands
 *    on the board underneath exactly as it would have. A flash that could
 *    swallow a move would be worse than no flash.
 *  - **Not the only copy.** Everything it says stays on the page afterwards —
 *    the results table, the status line, the match-over banner. Two seconds is
 *    missed by anyone who looked away, so nothing may depend on having seen it.
 *  - **Not a game's own vocabulary.** The headline is composed from labels the
 *    module named and numbers the module computed. Which *kind* of ending it was
 *    ("gin", "went out concealed", "the deck ran out") is printed underneath
 *    from the round's own facts, because only the module knows that.
 */

type Props = {
  visible: boolean;
  kind: FlashKind;
  log?: RoundLog;
  players: MatchPlayer[];
  standings?: Standing[];
  /** The module's own closing sentences — `view.status`. */
  status?: Fact[];
  /** Who won the match, from the match envelope. */
  winners?: string[];
  viewerId: string;
};

export function ResultsFlash({
  visible,
  kind,
  log,
  players,
  standings,
  status,
  winners,
  viewerId,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const stillness = useReducedMotion();
  const styles = useMemo(() => flashStyles(metrics, skin), [metrics, skin]);

  const lines = useMemo(
    () => compose({ kind, log, players, standings, status, winners, viewerId }),
    [kind, log, players, standings, status, winners, viewerId],
  );

  // Rendered separately from `visible` so the exit has somewhere to happen: the
  // caller flips visible off at the end of the hold, and this stays mounted for
  // as long as the fade takes.
  const [rendered, setRendered] = useState(false);
  const opacity = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (visible) {
      setRendered(true);
      const anim = Animated.timing(opacity, {
        toValue: 1,
        // Someone who asked their system for stillness is still told the round
        // ended — they are told by its being there, not by its arrival.
        duration: stillness ? 0 : ms(180),
        easing: ARRIVAL_EASING,
        useNativeDriver: true,
      });
      anim.start();
      return () => anim.stop();
    }
    if (!rendered) return;
    const anim = Animated.timing(opacity, {
      toValue: 0,
      duration: stillness ? 0 : ms(220),
      easing: ARRIVAL_EASING,
      useNativeDriver: true,
    });
    anim.start(({ finished }) => {
      if (finished) setRendered(false);
    });
    return () => anim.stop();
  }, [visible, rendered, stillness, opacity]);

  // Two seconds of picture is not perceivable through a screen reader, so the
  // headline goes out through the one channel that is — and the drawing itself
  // is hidden from the accessibility tree rather than competing with it.
  useEffect(() => {
    if (visible && lines.headline) AccessibilityInfo.announceForAccessibility(lines.headline);
  }, [visible, lines.headline]);

  if (!rendered || !lines.headline) return null;

  const final = kind === 'match';

  return (
    <Animated.View
      pointerEvents="none"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={[StyleSheet.absoluteFill, styles.layer, { opacity }]}
      testID="results-flash"
    >
      {/* The end of a match takes the whole screen; the end of a round does
          not. A hand ending is a pause in play and the table should stay
          legible behind it — a match ending has nothing left to look at. */}
      <View style={[StyleSheet.absoluteFill, final ? styles.takeover : styles.veil]} />
      <View style={final ? styles.centreFinal : styles.card} testID="results-flash-card">
        <Text style={[styles.eyebrow, final && styles.eyebrowFinal]} testID="results-flash-eyebrow">
          {lines.eyebrow}
        </Text>
        <Text style={[styles.headline, final && styles.headlineFinal]} testID="results-flash-headline">
          {lines.headline}
        </Text>
        {lines.sub ? (
          <Text style={styles.sub} testID="results-flash-sub">
            {lines.sub}
          </Text>
        ) : null}
        {lines.delta ? (
          <Text style={styles.delta} testID="results-flash-delta">
            <Text style={lines.up ? styles.up : styles.down}>{lines.delta}</Text>
            {lines.after ? <Text style={styles.after}>{` · ${lines.after}`}</Text> : null}
          </Text>
        ) : null}
      </View>
    </Animated.View>
  );
}

type Lines = {
  eyebrow: string;
  headline: string;
  sub?: string;
  delta?: string;
  after?: string;
  up?: boolean;
};

/**
 * What the flash says, from what the server sent.
 *
 * Exported for its own test: this is the half that can be wrong in a way a
 * screenshot would not catch — a penalty printed as its negation, a winner named
 * by id, a round numbered off by one.
 */
export function compose({
  kind,
  log,
  players,
  standings,
  status,
  winners,
  viewerId,
}: Omit<Props, 'visible'>): Lines {
  const names = (ids: string[]) => ids.map((id) => playerName(players, id)).join(', ');

  if (kind === 'match') {
    const won = winners ?? [];
    const headline = !won.length
      ? t('flash.matchDrawn')
      : won.includes(viewerId)
        ? t('flash.matchWonYou')
        : t('flash.matchWon', { winners: names(won) });
    // The final numbers, in the module's own unit and the module's own
    // direction — `shownScore` is what keeps a rummy penalty from printing as
    // the negation the server ranks on.
    const sub = standings?.length
      ? standings.map((s) => `${playerName(players, s.playerId)} ${shownScore(s)}`).join('  ·  ')
      : sentenceOf(status, players);
    return { eyebrow: t('flash.matchOver'), headline, sub };
  }

  const last = log?.rounds[log.rounds.length - 1];
  if (!last) return { eyebrow: '', headline: '' };

  const took = last.winners ?? [];
  const headline = !took.length
    ? t('flash.roundDrawn')
    : took.includes(viewerId)
      ? t('flash.roundWonYou')
      : t('flash.roundWon', { winners: names(took) });

  // What kind of ending it was, in the module's own words. A round with nothing
  // to say about itself simply has no sub-line.
  const sub = (last.facts ?? []).map((f) => factText(f, players)).join(' · ') || undefined;

  const mine = last.scores.find((s) => s.playerId === viewerId);
  const moved = mine ? (mine.shown ?? mine.delta) : undefined;
  const total = mine ? (mine.shownTotal ?? mine.total) : undefined;

  return {
    // "Hand 3", "Deal 2" — what this game calls a round, and which one.
    eyebrow: `${label(log!.labelKey)} ${last.number}`.trim(),
    headline,
    sub,
    delta: moved === undefined ? undefined : moved > 0 ? `+${moved}` : String(moved),
    after: total === undefined ? undefined : t('flash.nowOn', { total }),
    up: (moved ?? 0) >= 0,
  };
}

/** The module's own closing line, where there is one. */
function sentenceOf(status: Fact[] | undefined, players: MatchPlayer[]): string | undefined {
  const first = status?.[0];
  return first ? factText(first, players) : undefined;
}

function flashStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    layer: { alignItems: 'center', justifyContent: 'center', paddingHorizontal: 18 },
    // A round ending: the board stays readable underneath.
    veil: { backgroundColor: 'rgba(3, 12, 7, 0.55)' },
    // A match ending: nothing left behind it worth looking at.
    takeover: { backgroundColor: colors.bg, opacity: 0.96 },
    card: {
      // Centred by the layer, not stretched by it: `alignSelf: 'stretch'` wins
      // over the parent's `alignItems: 'center'`, so the card filled the cross
      // axis, `maxWidth` clipped it back to 420, and it came to rest hard
      // against the left edge of a 1280-wide screen.
      alignSelf: 'center',
      maxWidth: 420,
      width: '100%',
      alignItems: 'center',
      gap: 5,
      paddingVertical: 18,
      paddingHorizontal: 20,
      borderRadius: 14,
      borderWidth: 1,
      borderColor: colors.accent,
      backgroundColor: colors.surface,
    },
    centreFinal: { alignItems: 'center', gap: 7, maxWidth: 460 },
    eyebrow: {
      color: colors.accent,
      fontSize: 11 * m.scale,
      fontWeight: '700',
      letterSpacing: 1.6,
      textTransform: 'uppercase',
      textAlign: 'center',
    },
    eyebrowFinal: { color: colors.gold },
    headline: {
      color: colors.text,
      fontSize: 26 * m.scale,
      lineHeight: 30 * m.scale,
      fontWeight: '700',
      textAlign: 'center',
    },
    headlineFinal: { fontSize: 34 * m.scale, lineHeight: 39 * m.scale },
    sub: { color: colors.muted, fontSize: 14 * m.scale, textAlign: 'center' },
    delta: { fontSize: 15 * m.scale, textAlign: 'center', marginTop: 2 },
    up: { color: colors.success, fontWeight: '700' },
    down: { color: colors.danger, fontWeight: '700' },
    after: { color: colors.muted, fontWeight: '400' },
  });
}
