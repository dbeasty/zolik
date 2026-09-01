import { useMemo } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';

import { CardBack } from '@/src/components/CardBack';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { parseCard } from '@/src/lib/cards';
import type { CardMetrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * How much room a card takes, at scale 1 — the size the sizes in
 * `src/lib/layout.ts` are derived from. Exported because a couple of call
 * sites need the *unscaled* metric for something other than drawing a card
 * (see `useDropRegistry`'s HIT_SLOP comment); anything that draws needs the
 * scaled numbers from `useMetrics().card` instead, not this.
 */
export const CARD_METRICS = {
  width: 52,
  height: 72,
  gap: 6,
  ringPadding: 1,
  ringBorder: 2,
  compactWidth: 44,
  compactHeight: 60,
};

type Props = {
  card: string;
  selected?: boolean;
  // Marks the card(s) just picked up from the deck or discard pile this
  // turn, so it's obvious which card is new — independent of `selected`
  // (a drawn card can be both, e.g. right after tapping it to stage a meld).
  justDrawn?: boolean;
  // The floating copy of a card currently being dragged — a distinct ring
  // color from justDrawn/selected so "this is what my finger is holding
  // right now" reads as its own state, not a reused one.
  dragging?: boolean;
  onPress?: () => void;
  compact?: boolean;
  // A card in a stacked meld shows only its top corner — the rest is under
  // the next card in the pile. The rank and suit need to share that corner
  // side by side, rather than the rank on top and the suit centered below,
  // or the suit is exactly what the overlap crops off.
  stacked?: boolean;
  // The card carries a mark from the module — something true about this card
  // that the player should act on before it becomes a refusal. Drawn as its
  // own ring colour, not a reuse of justDrawn's: "this card is new" and "this
  // card owes your lay-down" are different facts, and the second one is the
  // one with a consequence.
  badged?: boolean;
  // Passed straight through to the outer wrapper (data-testid on web) — set
  // by the caller, which knows the card's context (hand index, staged
  // group, table meld), since CardView itself doesn't.
  testID?: string;
  // Shown back-up, in the same ring-wrapped chassis as a face — so a card
  // turned over occupies exactly the box its face would, to the pixel, and
  // nothing measured around it moves. A ceremonial turn, not a secret: the
  // server still says which card it is (see matchTypes.CardView.faceDown).
  faceDown?: boolean;
};

/** The court cards get a medallion on a rich face rather than a giant pip. */
const COURT_RANKS = new Set(['J', 'Q', 'K']);

/** How much of a card's own side shows below and to the right of it. */
const EDGE = 2;

/** Every dimension a card's own render needs, computed once per card size and skin. */
function cardStyles(m: CardMetrics, s: Skin) {
  const colors = s.colors;
  const card = s.card;
  // The rich face uses smaller indices than the plain face's single rank,
  // because it shows two of them plus a centre pip in the same 52×72.
  const cornerRankFont = Math.max(9, m.rankFont - 3);
  const cornerSuitFont = Math.max(8, m.suitFont - 9);
  const medallionSize = Math.round(m.width * 0.52);
  return StyleSheet.create({
    ring: {
      borderRadius: 8,
      borderWidth: m.ringBorder,
      borderColor: 'transparent',
      padding: m.ringPadding,
    },
    // The card's own thickness: a sliver of its shaded edge showing below and
    // to the right, the way a card lying on felt shows the side nobody
    // printed on. Absolutely positioned, so it adds nothing to the box a drop
    // is measured against — the same contract the shadow keeps, and on the
    // same switch as the bevel, so face, back and edge agree about the light.
    cardEdge: {
      position: 'absolute',
      left: m.ringPadding + EDGE,
      top: m.ringPadding + EDGE,
      width: m.width,
      height: m.height,
      borderRadius: 6,
      backgroundColor: card.bevel?.shadow ?? 'transparent',
    },
    cardEdgeCompact: { width: m.compactWidth, height: m.compactHeight },
    justDrawnRing: { borderColor: colors.success },
    badgedRing: { borderColor: colors.gold, borderStyle: 'dashed' },
    draggingRing: { borderColor: colors.accent },
    card: {
      width: m.width,
      height: m.height,
      backgroundColor: colors.cardBg,
      borderRadius: 6,
      borderWidth: 2,
      borderColor: colors.cardBorder,
      padding: 4,
      marginRight: m.gap,
      justifyContent: 'space-between',
    },
    // Shadow, not size: the card's box is identical with or without it, so a
    // skin that lifts cards off the felt never moves a drop measurement.
    cardShadow: {
      shadowColor: '#000',
      shadowOffset: { width: 0, height: 2 },
      shadowOpacity: 0.3,
      shadowRadius: 3,
      elevation: 3,
    },
    // The floating copy under a finger sits visibly *above* the board: a
    // touch larger, a touch tilted, a deeper shadow. Transform only — the
    // slot it left keeps its measured size to the pixel.
    cardDragging: {
      transform: [{ scale: 1.06 }, { rotate: '2deg' }],
      shadowColor: '#000',
      shadowOffset: { width: 0, height: 8 },
      shadowOpacity: 0.45,
      shadowRadius: 12,
      elevation: 8,
    },
    // Fake thickness: lit top/left edges, shaded bottom/right ones. Border
    // colours only, on the same 2px border the card always has — a card
    // with depth is still exactly the size of a card without it.
    cardBevel: card.bevel
      ? {
          borderTopColor: card.bevel.highlight,
          borderLeftColor: card.bevel.highlight,
          borderBottomColor: card.bevel.shadow,
          borderRightColor: card.bevel.shadow,
        }
      : {},
    // The back in the same chassis a face sits in — see the faceDown prop.
    backBox: { marginRight: m.gap },
    // A rich face positions its own corners; the plain face keeps the padded
    // column layout it has always had.
    cardRich: { padding: 0 },
    faceFill: {
      position: 'absolute',
      top: 0,
      bottom: 0,
      left: 0,
      right: 0,
      borderRadius: 4,
    },
    compact: {
      width: m.compactWidth,
      height: m.compactHeight,
    },
    selected: {
      borderColor: colors.gold,
      backgroundColor: card.selectedFace,
    },
    joker: { backgroundColor: card.jokerFace },
    pressed: { opacity: 0.85, transform: [{ scale: 0.96 }] },
    rank: {
      fontSize: m.rankFont,
      fontWeight: '700',
      color: card.ink,
    },
    // "JKR" is 3 characters vs. 1-2 for every other rank, so it needs its own
    // (smaller) size to stay inside the card instead of overflowing the edge.
    jokerRank: { fontSize: m.jokerRankFont },
    suit: {
      fontSize: m.suitFont,
      alignSelf: 'center',
      color: card.ink,
    },
    corner: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 2,
    },
    suitInline: {
      fontSize: m.suitInlineFont,
      color: card.ink,
    },
    red: { color: card.red },
    // ---- The rich face: two indices and a centre. ----
    cornerTL: {
      position: 'absolute',
      top: 2,
      left: 3,
      alignItems: 'center',
    },
    cornerBR: {
      position: 'absolute',
      bottom: 2,
      right: 3,
      alignItems: 'center',
      // A real card reads from either end of the table.
      transform: [{ rotate: '180deg' }],
    },
    cornerRank: {
      fontSize: cornerRankFont,
      lineHeight: cornerRankFont + 1,
      fontWeight: '700',
      color: card.ink,
    },
    cornerSuit: {
      fontSize: cornerSuitFont,
      lineHeight: cornerSuitFont + 1,
      color: card.ink,
    },
    center: {
      position: 'absolute',
      top: 0,
      bottom: 0,
      left: 0,
      right: 0,
      alignItems: 'center',
      justifyContent: 'center',
    },
    centerPip: {
      fontSize: m.suitFont + 6,
      color: card.ink,
    },
    medallion: {
      width: medallionSize,
      height: medallionSize,
      borderRadius: medallionSize / 2,
      borderWidth: 1.5,
      alignItems: 'center',
      justifyContent: 'center',
      borderColor: card.ink,
    },
    medallionRed: { borderColor: card.red },
    medallionRank: {
      fontSize: Math.round(medallionSize * 0.5),
      lineHeight: Math.round(medallionSize * 0.58),
      fontWeight: '700',
      color: card.ink,
    },
    medallionSuit: {
      fontSize: Math.max(7, Math.round(medallionSize * 0.28)),
      lineHeight: Math.max(8, Math.round(medallionSize * 0.32)),
      color: card.ink,
    },
    jokerStar: {
      fontSize: m.suitFont + 8,
      color: card.red,
    },
  });
}

export function CardView({
  card,
  selected,
  justDrawn,
  dragging,
  badged,
  onPress,
  compact,
  stacked,
  testID,
  faceDown,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const d = parseCard(card);
  // Recomputed only when the card's own size or the skin changes — every
  // other render of a card reuses the same style objects.
  const styles = useMemo(() => cardStyles(metrics.card, skin), [metrics.card, skin]);

  if (faceDown) {
    // The same ring wrapper a face gets, so the turned card's box is
    // identical to its neighbours' — the back inside it draws its own
    // border, exactly the size of the face's.
    return (
      <View testID={testID} style={styles.ring}>
        <View style={styles.backBox}>
          <CardBack
            width={compact ? metrics.card.compactWidth : metrics.card.width}
            height={compact ? metrics.card.compactHeight : metrics.card.height}
          />
        </View>
      </View>
    );
  }

  const rich = skin.card.face === 'rich' && !stacked;
  // The gradient wash is the resting face only: a selected or joker card
  // shows its own solid fill, and painting the wash over it would hide the
  // one thing those fills are for.
  const washed = rich && !!skin.card.faceGradient && !selected && !d.isJoker;

  const face = stacked ? (
    <View style={styles.corner}>
      <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
        {d.rank}
      </Text>
      <Text style={[styles.suitInline, d.isRed && styles.red]}>{d.suitSymbol}</Text>
    </View>
  ) : rich ? (
    <>
      <View style={styles.center} pointerEvents="none">
        {d.isJoker ? (
          <Text style={styles.jokerStar}>★</Text>
        ) : COURT_RANKS.has(d.rank) ? (
          <View style={[styles.medallion, d.isRed && styles.medallionRed]}>
            <Text style={[styles.medallionRank, d.isRed && styles.red]}>{d.rank}</Text>
            <Text style={[styles.medallionSuit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
          </View>
        ) : (
          <Text style={[styles.centerPip, d.isRed && styles.red]}>{d.suitSymbol}</Text>
        )}
      </View>
      <View style={styles.cornerTL}>
        <Text style={[styles.cornerRank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
          {d.rank}
        </Text>
        {d.isJoker ? null : (
          <Text style={[styles.cornerSuit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
        )}
      </View>
      {d.isJoker ? null : (
        <View style={styles.cornerBR}>
          <Text style={[styles.cornerRank, d.isRed && styles.red]}>{d.rank}</Text>
          <Text style={[styles.cornerSuit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
        </View>
      )}
    </>
  ) : (
    <>
      <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
        {d.rank}
      </Text>
      <Text style={[styles.suit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
    </>
  );

  const content = (
    // Ring wrapper is always present at a fixed size (border color just
    // toggles transparent<->success/accent) so highlighting a card never
    // nudges its neighbors' layout — a card that shifts mid-gesture is
    // exactly what broke double-tap-to-discard before this was fixed.
    <View
      testID={testID}
      // Selectedness said out loud rather than only drawn. A gold border is
      // invisible to a screen reader, and it is also the only evidence a test
      // could otherwise check — which would mean asserting on a hex colour,
      // and those change for design reasons that have nothing to do with
      // whether the card is picked.
      //
      // `aria-selected` rather than `accessibilityState={{selected}}`: React
      // Native has taken the aria spelling since 0.71 and maps it back to
      // accessibilityState on iOS and Android, while react-native-web forwards
      // it to the DOM. The older spelling reaches native but this version of
      // react-native-web drops it, so it would leave the web build silently
      // saying nothing.
      aria-selected={!!selected}
      style={[
        styles.ring,
        // Later wins, so this is the precedence read backwards: what your
        // finger is holding beats what you owe, which beats what just
        // arrived. A card taken off the discard pile is all three at once,
        // and "you owe this to your lay-down" is the one with a consequence.
        justDrawn && styles.justDrawnRing,
        badged && styles.badgedRing,
        dragging && styles.draggingRing,
      ]}
    >
      {/* Drawn before the card, so the card lies on top of its own edge. */}
      {skin.card.bevel ? (
        <View pointerEvents="none" style={[styles.cardEdge, compact && styles.cardEdgeCompact]} />
      ) : null}
      <View
        style={[
          styles.card,
          rich && styles.cardRich,
          skin.card.shadow && styles.cardShadow,
          // Not on a selected card: its solid selection border must win, and
          // per-side colours would beat an all-side one regardless of order.
          !selected && styles.cardBevel,
          compact && styles.compact,
          selected && styles.selected,
          d.isJoker && styles.joker,
          dragging && styles.cardDragging,
        ]}
      >
        {washed ? (
          <LinearGradient colors={skin.card.faceGradient!} style={styles.faceFill} />
        ) : null}
        {face}
      </View>
    </View>
  );
  if (onPress) {
    return (
      <Pressable onPress={onPress} style={({ pressed }) => pressed && styles.pressed}>
        {content}
      </Pressable>
    );
  }
  return content;
}
