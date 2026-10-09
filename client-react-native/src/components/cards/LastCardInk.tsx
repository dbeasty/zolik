import { StyleSheet, Text, View, type StyleProp, type TextStyle } from 'react-native';

import type { Fact } from '@/src/api/matchTypes';
import { DUSK, INKS } from '@/src/components/cards/lastCardArt';
import { lastCardColourOfKey, lastCardInkRuns } from '@/src/lib/cards';
import { t } from '@/src/lib/i18n';
import { label } from '@/src/lib/labels';

/**
 * A line of text with every Last Card it names printed in that card's own
 * colour — "■4" in amber, "● Coral" in coral — so a log or a header reads the
 * way the table looks instead of in one flat ink. Lines from any other game
 * have no shapes in them and come back unchanged.
 *
 * Children of a `<Text>`: the caller keeps its own style, and only the
 * coloured stretches are restyled.
 */
export function LastCardInk({ text }: { text: string }) {
  const runs = lastCardInkRuns(text);
  if (runs.length === 1 && !runs[0].colour) return <>{text}</>;
  return (
    <>
      {runs.map((r, i) =>
        r.colour ? (
          <Text key={i} style={{ color: INKS[r.colour].main, fontWeight: '800' }}>
            {r.text}
          </Text>
        ) : (
          r.text
        ),
      )}
    </>
  );
}

/**
 * The header's "Colour in play", with the colour itself as a chip of that
 * colour — after a wild there is no card of it on the pile to look at.
 */
export function ColourInPlay({ fact, style }: { fact: Fact; style: StyleProp<TextStyle> }) {
  const colour = lastCardColourOfKey(fact.value)!;
  return (
    <View style={styles.row} testID="colour-in-play">
      <Text style={style}>{label(fact.labelKey)}</Text>
      <View style={[styles.chip, { backgroundColor: INKS[colour].main }]}>
        <Text style={styles.chipText}>{t(fact.value!)}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  chip: { borderRadius: 999, paddingHorizontal: 8, paddingVertical: 1 },
  chipText: { color: DUSK.field, fontSize: 12, fontWeight: '800' },
});
