import { StyleSheet, View } from 'react-native';

import { CardText as Text } from '@/src/components/cards/CardText';
import { TILE_GLYPH } from '@/src/lib/cards';

/**
 * A Rummy Tiles tile: a number in one of four inks, with a shape beside it so
 * colour is never the only signal. The tile's own colours, like Last Card's:
 * no skin repaints them. Sizes come from the card metrics, so a tile occupies
 * exactly the box a playing card does.
 */
export const TILE_INK: Record<string, string> = {
  R: '#C62828',
  B: '#1565C0',
  O: '#E67E00',
  K: '#1B1B1B',
};

type Props = {
  number: string;
  colour: string;
  stacked: boolean;
  indexFont: number;
  suitFont: number;
};

export function TileFace({ number, colour, stacked, indexFont, suitFont }: Props) {
  const ink = TILE_INK[colour] ?? '#1B1B1B';
  const glyph = TILE_GLYPH[colour] ?? '●';
  return (
    <View style={stacked ? styles.row : styles.col}>
      <Text style={{ fontSize: indexFont, lineHeight: Math.round(indexFont * 1.05), fontWeight: '700', color: ink }}>
        {number}
      </Text>
      <Text style={{ fontSize: suitFont, lineHeight: Math.round(suitFont * 1.1), color: ink }}>{glyph}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  col: { alignItems: 'flex-start' },
  row: { flexDirection: 'row', alignItems: 'center', gap: 2 },
});
