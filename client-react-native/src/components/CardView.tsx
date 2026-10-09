import { useMemo } from 'react';
import { Platform, Pressable, StyleSheet, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';

import { cardSpokenName } from '@/src/a11y/cardNames';
// Card type, pinned to the card's own size — see CardText.
import { CardText as Text } from '@/src/components/cards/CardText';
import { CardBack } from '@/src/components/CardBack';
import { DeluxeFace } from '@/src/components/cards/DeluxeFace';
import { GermanFace } from '@/src/components/cards/GermanFace';
import { GermanSuit, germanInk } from '@/src/components/cards/GermanSuit';
import { LastCardFace, lastCardSetFor } from '@/src/components/cards/LastCardFace';
import { isLastCardCode } from '@/src/components/cards/lastCardArt';
import { Suit } from '@/src/components/cards/Suit';
import { VectorFace } from '@/src/components/cards/VectorFace';
import { PRINTED } from '@/src/cards/vector/faces';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useFourColour, useSkin } from '@/src/hooks/useSkin';
import { parseCard } from '@/src/lib/cards';
import { germanIndex, useDeck } from '@/src/lib/deck';
import { isCourt } from '@/src/lib/pips';
import { CARD_BORDER, INDEX_PADDING, type CardMetrics } from '@/src/lib/layout';
import { fourColourSkin, germanFourColourInk } from '@/src/skins/fourColour';
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
  /** What the card stands for in play, where its face can show it (`CardView.as` on the wire). */
  as?: string;
  /**
   * How the card presents itself to assistive technology, where the caller
   * knows more than the card does — its place in a hand, that it is one of a
   * listbox's options. Absent, a card describes itself: a face is an image
   * named in words ("Seven of Clubs"), a back is hidden (whoever holds the
   * backs says how many there are), and a card with an `onPress` is a toggle
   * button with the same name. See `CardA11y`.
   */
  a11y?: CardA11y;
  /** Set by `Tip` on the web: the id of the description to link. */
  'aria-describedby'?: string;
};

/**
 * What a caller can tell a card about how it is read.
 *
 * Applied to the ring — the element carrying the card's `testID` — on the
 * web, so that the element a test finds by id is the element a screen reader
 * lands on. On iOS and Android a card in a hand is announced by the hand's
 * own wrapper instead (see `HandZone`), and `hidden` is all that applies.
 */
export type CardA11y = {
  /**
   * `option` for a card in a hand drawn as a listbox; `img` for a card that
   * is only looked at; `none` for a card inside a control that already names
   * it (a meld's toggle), which must not be read twice.
   */
  role?: 'option' | 'img' | 'none';
  label?: string;
  /** Taken out of the accessibility tree entirely. */
  hidden?: boolean;
  /** For `option`: the roving tab stop — 0 on the one card the hand's Tab lands on, -1 on the rest. */
  tabIndex?: 0 | -1;
  onKeyDown?: (e: { key: string; shiftKey?: boolean; preventDefault: () => void; stopPropagation: () => void }) => void;
  onFocus?: () => void;
  onBlur?: () => void;
};

/** The hand on the web: an option pressed directly says it can be (see `ringA11y`). */
const webPointer = { cursor: 'pointer' } as const;

/** How much of a card's own side shows below and to the right of it. */
const EDGE = 2;

/**
 * The card's own border, on every face. A face drawn *inside* the card has to
 * subtract it twice to know how much room it actually has — `width` is the
 * outer box (React Native measures border-box), and a pip laid out against
 * the outer box lands under the border.
 *
 * It comes from the layout metrics rather than from here because the plain
 * face's index is sized against the strip a fanned card shows, and that sum
 * has to subtract exactly this border. Two files holding one number is how
 * the fan and the index would come to disagree.
 */
const BORDER = CARD_BORDER;

/** The narrowest card a Last Card's full, illustrated face is drawn on. */
const LAST_CARD_FULL_WIDTH = 64;

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
      borderWidth: BORDER,
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
    // A rich face positions its own corners; the plain face pads by its own
    // (smaller) margin, because on that face every pixel of the peek strip
    // that is not air is index.
    cardRich: { padding: 0 },
    cardPlain: { padding: INDEX_PADDING },
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
    corner: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 2,
    },
    // Stacked, not side by side: this has to fit in the strip of a card that
    // the next card is covering, and a strip is about as wide as one glyph.
    plainIndex: { alignItems: 'flex-start' },
    // The plain face's whole content, and therefore as large as the strip it
    // is read in allows — `indexFont` is that sum, done in the metrics so the
    // fan and the index cannot drift. Tight line heights because the two of
    // them are one mark, the way a printed index is.
    indexRank: {
      fontSize: m.indexFont,
      lineHeight: Math.round(m.indexFont * 1.05),
      fontWeight: '700',
      color: card.ink,
    },
    // The index is sized for "10". "JKR" is three characters, so it is set
    // smaller to take the same strip rather than to overflow it.
    indexJokerRank: {
      fontSize: Math.max(9, Math.round(m.indexFont * 0.52)),
      lineHeight: Math.max(10, Math.round(m.indexFont * 0.6)),
    },
    indexSuit: {
      fontSize: m.indexSuitFont,
      lineHeight: Math.round(m.indexSuitFont * 1.1),
      color: card.ink,
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
  as,
  a11y,
  'aria-describedby': describedBy,
}: Props) {
  const metrics = useMetrics();
  const deck = useDeck();
  const d = parseCard(card);
  // The four-colour deck is this skin with a diamond's red turned blue and a
  // club's black turned green — see `fourColourSkin`. French faces only: the
  // German pack's index takes its own four inks below, and Last Card is
  // four-coloured already.
  const tableSkin = useSkin();
  const fourColour = useFourColour();
  const skin = fourColour && deck === 'french' ? fourColourSkin(tableSkin, d.suit) : tableSkin;
  // Recomputed only when the card's own size or the skin changes — every
  // other render of a card reuses the same style objects.
  const styles = useMemo(() => cardStyles(metrics.card, skin), [metrics.card, skin]);

  if (faceDown) {
    // The same ring wrapper a face gets, so the turned card's box is
    // identical to its neighbours' — the back inside it draws its own
    // border, exactly the size of the face's.
    // A back says nothing on its own: what it is worth knowing — how many
    // there are — belongs to whoever holds them, and is said there.
    const backA11y =
      a11y?.label && !a11y.hidden
        ? Platform.OS === 'web'
          ? { role: 'img', 'aria-label': a11y.label }
          : { accessible: true, accessibilityRole: 'image', accessibilityLabel: a11y.label }
        : Platform.OS === 'web'
          ? { 'aria-hidden': true }
          : { importantForAccessibility: 'no-hide-descendants', accessibilityElementsHidden: true };
    return (
      <View testID={testID} style={styles.ring} {...(backA11y as object)}>
        <View style={styles.backBox}>
          <CardBack
            width={compact ? metrics.card.compactWidth : metrics.card.width}
            height={compact ? metrics.card.compactHeight : metrics.card.height}
          />
        </View>
      </View>
    );
  }

  // A meld's overlapped cards show a strip of one edge and nothing else, so
  // there is nowhere for a pip arrangement or a drawn court figure to go —
  // these two faces sit out a stacked card and it falls back to a corner.
  // The engraved deck is the exception, for the reason below it.
  const deluxe = skin.card.face === 'deluxe' && !stacked;
  const rich = skin.card.face === 'rich' && !stacked;
  /**
   * The engraved deck draws a *stacked* card too, at every width.
   *
   * A meld's cards overlap downwards, so each one shows a strip of its own
   * top edge — and on this deck that strip is the card's own engraved index,
   * because the index is part of the art rather than type laid over it. A
   * fanned meld then looks like a fanned meld: the same card, cropped, rather
   * than a rank and a suit typed into an empty corner.
   *
   * On a phone that strip is 26px off a 60px card, which is small — and it is
   * still the whole of what a meld has to say. A melded card is not one you
   * act on: it is already down, and what the strip has to carry is *which
   * card it is*, once, at a glance. The printed corner does that at the size
   * printers chose for exactly this, which is why a real deck is readable
   * fanned in a hand. Shipping one face at both widths also means a meld and
   * the hand above it are drawn from the same deck rather than from a deck
   * and a font.
   */
  const vector = skin.card.face === 'vector';
  // Everything else: one index, drawn against the card's own edge. A stacked
  // card keeps the old corner and the old padding, because what it has to fit
  // in is a strip of its *top* edge rather than of its left one.
  const plain = !vector && !deluxe && !rich && !stacked;
  // The gradient wash is the resting face only: a selected or joker card
  // shows its own solid fill, and painting the wash over it would hide the
  // one thing those fills are for.
  /**
   * A German-suited table (`src/lib/deck.ts`) draws the same codes from the
   * German pack. Every skin's face style has a German counterpart: the plain
   * index for `plain`, the full printed face for the other three, and the
   * corner for a stacked card — so a skin still chooses *how much* of a card
   * is drawn, and the deck chooses *which* card that is.
   */
  const german = deck === 'german' && !d.isJoker;
  /**
   * Last Card's own pack: codes no other deck has, so its faces are its own
   * too. The skin still chooses how much is drawn — the plain skin and a
   * stacked card get the frame-and-index face, every other skin the full one.
   */
  const lastCard = deck === 'lastcard' && isLastCardCode(card);
  /**
   * Which Last Card face. Each skin has its own set of the pack —
   * `lastCardSetFor`. A stacked card is the small face — frame and one big
   * index — in every set; the classic set, which is the condensed one, also
   * uses it on a compact board and a phone's narrow cards. The casino and
   * heirloom sets draw their whole face wherever a card is shown whole, the
   * pile included.
   */
  const lastCardSet = lastCardSetFor(skin);
  const lastCardVariant = stacked
    ? 'plain'
    : lastCardSet === 'classic'
      ? compact || metrics.card.width < LAST_CARD_FULL_WIDTH
        ? 'plain'
        : 'classic'
      : lastCardSet === 'heirloom'
        ? 'heirloom'
        : 'full';
  const germanFull = german && !stacked && !plain;
  const washed =
    !lastCard &&
    (rich || deluxe || vector || germanFull) && !!skin.card.faceGradient && !selected && !d.isJoker;
  const germanLabel = german ? germanIndex(d.rank) : '';
  const germanColor = german
    ? (fourColour && germanFourColourInk(skin, d.suit)) || germanInk(d.suit, skin.card.red)
    : '';

  const face = lastCard ? (
    <LastCardFace
      card={card}
      as={as}
      variant={lastCardVariant}
      width={(compact ? metrics.card.compactWidth : metrics.card.width) - 2 * BORDER}
      height={(compact ? metrics.card.compactHeight : metrics.card.height) - 2 * BORDER}
      indexFont={metrics.card.indexFont}
    />
  ) : german ? (
    stacked ? (
      <View style={styles.corner}>
        <Text style={[styles.rank, { color: germanColor }]}>{germanLabel}</Text>
        <GermanSuit suit={d.suit} size={metrics.card.suitInlineFont} red={skin.card.red} />
      </View>
    ) : plain ? (
      <View style={styles.plainIndex}>
        <Text
          style={[
            styles.indexRank,
            { color: germanColor },
            // "Sv" is two letters where the index is sized for one; "10" is
            // what it was sized for, so it keeps the full size.
            germanLabel.length > 1 &&
              d.rank !== '10' && { fontSize: Math.round(metrics.card.indexFont * 0.78) },
          ]}
        >
          {germanLabel}
        </Text>
        <GermanSuit suit={d.suit} size={metrics.card.indexSuitFont} red={skin.card.red} />
      </View>
    ) : (
      <GermanFace
        card={d}
        width={(compact ? metrics.card.compactWidth : metrics.card.width) - 2 * BORDER}
        height={(compact ? metrics.card.compactHeight : metrics.card.height) - 2 * BORDER}
        ink={skin.card.ink}
        red={skin.card.red}
        stock={selected ? skin.card.selectedFace : skin.colors.cardBg}
      />
    )
  ) : vector ? (
    <VectorFace
      card={d}
      width={(compact ? metrics.card.compactWidth : metrics.card.width) - 2 * BORDER}
      height={(compact ? metrics.card.compactHeight : metrics.card.height) - 2 * BORDER}
      palette={skin.card.cardPalette ?? PRINTED}
    />
  ) : stacked ? (
    <View style={styles.corner}>
      <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
        {d.rank}
      </Text>
      {/* The one corner a stacked meld shows. Under the deluxe skin it is the
          drawn suit rather than the font's, so a card half-hidden in a meld
          and the same card in hand are printed from the same shape. */}
      {skin.card.face === 'deluxe' && !d.isJoker ? (
        <Suit
          suit={d.suit}
          size={metrics.card.suitInlineFont}
          color={d.isRed ? skin.card.red : skin.card.ink}
        />
      ) : (
        <Text style={[styles.suitInline, d.isRed && styles.red]}>{d.suitSymbol}</Text>
      )}
    </View>
  ) : deluxe ? (
    <DeluxeFace
      card={d}
      width={(compact ? metrics.card.compactWidth : metrics.card.width) - 2 * BORDER}
      height={(compact ? metrics.card.compactHeight : metrics.card.height) - 2 * BORDER}
      ink={skin.card.ink}
      red={skin.card.red}
      courtAccent={skin.card.courtAccent}
      // The fill this card actually has, which the court figure draws its own
      // features in — a selected card and a joker are not on plain stock.
      stock={selected ? skin.card.selectedFace : d.isJoker ? skin.card.jokerFace : skin.colors.cardBg}
    />
  ) : rich ? (
    <>
      <View style={styles.center} pointerEvents="none">
        {d.isJoker ? (
          <Text style={styles.jokerStar}>★</Text>
        ) : /* A rich face gives the courts a medallion rather than a giant pip.
               Which ranks those are is `pips.ts`'s to say, so the deluxe face
               and this one can never disagree about what a court card is. */
        isCourt(d.rank) ? (
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
    /* One index, as big as the card can carry, and nothing else.
       This face used to draw its rank small in the corner and a large pip in
       the middle. Both halves were wrong on a phone: a closed hand covers all
       but a strip down each card's left edge, so the pip in the middle is the
       first thing the next card hides — it was decoration you paid for in the
       one dimension a phone has none of — and the rank left holding the card
       on its own was set for a face that had something else on it. So the pip
       goes, and the index takes the room it was using. */
    <View style={styles.plainIndex}>
      <Text
        style={[styles.indexRank, d.isJoker && styles.indexJokerRank, d.isRed && styles.red]}
      >
        {d.rank}
      </Text>
      {/* A joker has no suit. Its ★ was the centre pip, and it goes the way
          every other centre pip did — "JKR" on cream stock is the card. */}
      {d.isJoker ? null : (
        <Text style={[styles.indexSuit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
      )}
    </View>
  );

  // Who speaks for this card, and how — see `CardA11y`. Selectedness is
  // always written down as `data-selected` as well, for the tests and the
  // stylesheet-free checks that used to read `aria-selected` off every card:
  // the ARIA spelling is only allowed on an element whose role has a
  // selected state, which on this board is a card in a hand.
  const spoken = a11y?.label ?? cardSpokenName(card);
  const role = a11y?.role ?? (onPress ? 'none' : 'img');
  const ringA11y =
    Platform.OS === 'web'
      ? {
          dataSet: { selected: selected ? 'true' : 'false' },
          ...(a11y?.hidden
            ? { 'aria-hidden': true }
            : role === 'option'
              ? {
                  role: 'option',
                  'aria-label': spoken,
                  'aria-selected': !!selected,
                  'aria-describedby': describedBy,
                  tabIndex: a11y?.tabIndex ?? -1,
                  onKeyDown: a11y?.onKeyDown,
                  onFocus: a11y?.onFocus,
                  onBlur: a11y?.onBlur,
                  // The option is its own press target on the web: a
                  // pressable around it would put a second, unnamed focusable
                  // element inside the listbox, which a listbox may not hold.
                  // A click is what react-native-web's Pressable answers to as
                  // well, so nothing about a tap changes.
                  onClick: onPress,
                }
              : role === 'img'
                ? { role: 'img', 'aria-label': spoken, 'aria-describedby': describedBy }
                : {}),
        }
      : a11y?.hidden
        ? { importantForAccessibility: 'no-hide-descendants', accessibilityElementsHidden: true }
        : role === 'img'
          ? {
              accessible: true,
              accessibilityRole: 'image',
              accessibilityLabel: spoken,
              accessibilityState: { selected: !!selected },
            }
          : {};

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
      // Said as `aria-selected` only where the role has a selected state (a
      // card in a hand, a listbox option) — on a plain element it is an ARIA
      // error a screen reader ignores — and always as `data-selected`, which
      // is what the end-to-end suite reads. See `ringA11y` above.
      {...(ringA11y as object)}
      style={[
        styles.ring,
        // Later wins, so this is the precedence read backwards: what your
        // finger is holding beats what you owe, which beats what just
        // arrived. A card taken off the discard pile is all three at once,
        // and "you owe this to your lay-down" is the one with a consequence.
        justDrawn && styles.justDrawnRing,
        badged && styles.badgedRing,
        dragging && styles.draggingRing,
        role === 'option' && Platform.OS === 'web' && onPress ? webPointer : null,
      ]}
    >
      {/* Drawn before the card, so the card lies on top of its own edge. */}
      {skin.card.bevel ? (
        <View pointerEvents="none" style={[styles.cardEdge, compact && styles.cardEdgeCompact]} />
      ) : null}
      <View
        style={[
          styles.card,
          (rich || deluxe || germanFull) && styles.cardRich,
          plain && styles.cardPlain,
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
  if (onPress && role === 'option' && Platform.OS === 'web') return content;
  if (onPress) {
    // A card in a hand is pressed through its ring (the listbox option, which
    // holds the hand's one tab stop), so the pressable around it stays out of
    // the tab order. Anywhere else the pressable *is* the control: a toggle
    // button named by the card.
    const asOption = role === 'option';
    const pressA11y = asOption
      ? // On iOS and Android the hand's wrapper is the card's one accessible
        // element (see `HandZone`); a second, nested one would be read twice.
        { tabIndex: -1, accessible: false }
      : a11y?.role === 'none'
        ? {}
        : {
            accessibilityRole: 'button' as const,
            accessibilityLabel: spoken,
            'aria-pressed': !!selected,
            ...(Platform.OS === 'web' ? { 'aria-describedby': describedBy } : {}),
          };
    return (
      <Pressable onPress={onPress} style={({ pressed }) => pressed && styles.pressed} {...(pressA11y as object)}>
        {content}
      </Pressable>
    );
  }
  return content;
}
