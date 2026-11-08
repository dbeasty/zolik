import { StyleSheet, Text, View, type StyleProp, type TextStyle } from 'react-native';

import type { Fact } from '@/src/api/matchTypes';
import { Suit } from '@/src/components/cards/Suit';
import { useFourColour, useSkin } from '@/src/hooks/useSkin';
import { frenchSuitOfKey } from '@/src/lib/cards';
import { useDeck } from '@/src/lib/deck';
import { factText, type Named } from '@/src/lib/labels';
import { fourColourSkin } from '@/src/skins/fourColour';

/** The face colour when a skin does not wash its cards. */
const PLAIN_FACE = '#fbf8f1';

/**
 * A header fact that names a suit — trumps — drawn as that suit's pip on a
 * card-white pill, in the colour the cards print it. The suit decides every
 * trick, and as a grey word in a row of grey words it was the one fact on the
 * screen a player had to hunt for.
 *
 * The module opts in by sending the suit's key as the fact's value
 * (`suit.H`); the wording is still its own, read out in full beside the pip.
 * Sizes are fixed here, never a skin's — the skin only colours.
 */
export function SuitInPlay({
  fact,
  players,
  style,
}: {
  fact: Fact;
  players: Named[];
  style: StyleProp<TextStyle>;
}) {
  const suit = frenchSuitOfKey(fact.value)!;
  const tableSkin = useSkin();
  const fourColour = useFourColour();
  // The pip is the French one; a table drawn in another pack's suits shows
  // the pill and the words without a pip the cards do not carry.
  const french = useDeck() === 'french';
  const skin = fourColour && french ? fourColourSkin(tableSkin, suit) : tableSkin;
  const red = suit === 'H' || suit === 'D';
  const ink = red ? skin.card.red : skin.card.ink;
  const face = skin.card.faceGradient?.[0] ?? PLAIN_FACE;
  return (
    <View style={[styles.pill, { backgroundColor: face, borderColor: ink }]} testID={`suit-in-play-${suit}`}>
      {french ? <Suit suit={suit} size={18} color={ink} /> : null}
      <Text style={[style, styles.text, { color: ink }]}>{factText(fact, players)}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    borderRadius: 999,
    borderWidth: 1.5,
    paddingLeft: 6,
    paddingRight: 10,
    paddingVertical: 2,
  },
  text: { fontSize: 14, fontWeight: '800' },
});
