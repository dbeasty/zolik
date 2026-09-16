import { useMemo, type ComponentProps } from 'react';
import { StyleSheet, View } from 'react-native';
import Svg, { Circle, G, Path, Rect } from 'react-native-svg';

import type { Zone } from '@/src/api/matchTypes';
import { ZoneView } from '@/src/components/match/ZoneView';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import type { Metrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * The house, sat across the table.
 *
 * Blackjack has an opponent who is not a player: no seat, no stack, no
 * standing, and a hand that plays itself by a rule printed on the felt. The
 * board used to draw that hand as one more spread in the row of melds, next
 * to the players' — which is where it fits *structurally*, and is exactly
 * wrong to look at. The one adversary at the table appeared as an unlabelled
 * panel between two opponents' meld piles, and the question every blackjack
 * player is actually asking — what is the dealer showing? — had to be
 * answered by reading panel titles.
 *
 * So the dealer is drawn as what they are: a figure at the head of the table
 * with their cards in front of them, above the seats rather than among them.
 * Which zone that is comes from the server (`zone.dealer`), not from matching
 * a zone id or a game's name — see `matchTypes.Zone`.
 *
 * The cards themselves are a plain `ZoneView`, deliberately: the dealer's
 * hand flips, settles, counts and minimizes like every other zone on the
 * board, and re-implementing any of that here to put a face beside it would
 * be two zones that drift apart. This component is the face and the place.
 */

/**
 * Everything the zone inside needs — drop registration, panel state — taken
 * from `ZoneView`'s own props rather than re-listed here. Typed off the
 * component it is forwarded to, so a prop that changes shape there is a type
 * error here rather than a silently dropped drop target.
 */
type ForwardedZoneProps = Omit<ComponentProps<typeof ZoneView>, 'zone' | 'inline' | 'nested'>;

type Props = {
  zone: Zone;
  /** Passed straight through to the zone that draws the dealer's cards. */
  zoneProps?: ForwardedZoneProps;
};

export function Dealer({ zone, zoneProps }: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => dealerStyles(metrics), [metrics]);

  // The figure is sized off the seat avatar, so the house and the players are
  // drawn at one scale — and, like every size here, it comes from the metrics
  // rather than the skin. A roomy screen gets a noticeably larger one: the
  // dealer is the thing at the head of the table, and on a monitor there is
  // room to say so.
  const size = Math.round(metrics.seat.avatar * (metrics.roomy ? 1.6 : 1.15));

  return (
    <View style={styles.row} testID="dealer">
      {/* The figure alone, with no caption of its own: the panel beside it is
          titled with the zone's own label — "Dealer", in twenty-four
          languages already — and captioning the figure as well printed the
          word twice, side by side, along with the card count twice. The
          figure is what says "a person is sitting here"; the panel says whose
          cards those are. */}
      <View style={styles.who} testID="dealer-figure">
        <Croupier size={size} skin={skin} />
      </View>
      <View style={styles.cards}>
        <ZoneView zone={zone} inline nested {...(zoneProps ?? {})} />
      </View>
    </View>
  );
}

/**
 * The croupier: a bust in a waistcoat and a bow tie.
 *
 * Drawn in the skin's own colours rather than from the avatar palettes, and
 * that is the point of it. A player's face keeps its colours under every skin
 * precisely so two players can be told apart; the house is not a player, and
 * giving it one of those faces would put a thirteenth player at a table of
 * three. In the felt's own gold and dark it reads as part of the furniture,
 * which is what a dealer is.
 */
function Croupier({ size, skin }: { size: number; skin: Skin }) {
  const trim = skin.colors.gold;
  const dark = skin.card.ink;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      <Circle cx="50" cy="50" r="49" fill={skin.colors.surface} stroke={trim} strokeWidth="2" />
      <G>
        {/* Shoulders, then the shirt's V and a bow tie on top of it — the
            three shapes that say "dealer" and survive being 40px wide. */}
        <Path d="M12 100 C14 78 30 68 50 68 C70 68 86 78 88 100 Z" fill={dark} />
        <Path d="M40 69 L50 86 L60 69 C56 67 44 67 40 69 Z" fill={skin.colors.text} />
        <Path d="M50 74 L38 68 L38 80 Z" fill={trim} />
        <Path d="M50 74 L62 68 L62 80 Z" fill={trim} />
        <Rect x="46" y="71" width="8" height="6" rx="2" fill={trim} />
        {/* Head and a clean side-part, kept as a silhouette: any more detail
            at this size is noise, and the avatars next to it are silhouettes
            too. */}
        <Circle cx="50" cy="42" r="18" fill={skin.colors.text} />
        <Path d="M32 40 C32 26 68 26 68 40 C68 32 60 28 50 28 C40 28 32 32 32 40 Z" fill={dark} />
      </G>
    </Svg>
  );
}

function dealerStyles(m: Metrics) {
  return StyleSheet.create({
    // The house's side of the table: the figure, then their cards. Centred on
    // a roomy screen, where there is width to spare and the dealer reads as
    // sitting opposite; hard left on a phone, where centring just wastes the
    // only row there is.
    row: {
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: m.roomy ? 'center' : 'flex-start',
      gap: m.panel.gap + 4,
      marginTop: 10,
    },
    who: { alignItems: 'center' },
    // Sized to the cards rather than stretched, so the pair sits together in
    // the middle instead of the panel running to the far edge of a monitor.
    cards: { flexShrink: 1 },
  });
}
