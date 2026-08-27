import { StyleSheet, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';

import { useSkin } from '@/src/hooks/useSkin';

/**
 * The table itself: whatever the skin says is behind the board.
 *
 * One colour renders flat (the classic dark background). Two or more render
 * as the felt — a vertical wash that runs brighter through the middle band,
 * plus, when the skin asks for it, a darkening from the left and right
 * edges. Two linear gradients standing in for a radial one, which is as
 * close as the primitives here get to a table under a lamp.
 *
 * Absolutely positioned behind everything and `pointerEvents="none"`, so it
 * can never catch a touch or move a measurement.
 */
export function TableSurface() {
  const { table } = useSkin();
  const stops = table.background;

  if (stops.length < 2) {
    return (
      <View
        pointerEvents="none"
        style={[StyleSheet.absoluteFill, { backgroundColor: stops[0] }]}
      />
    );
  }

  return (
    <View pointerEvents="none" style={StyleSheet.absoluteFill}>
      <LinearGradient
        colors={stops as [string, string, ...string[]]}
        style={StyleSheet.absoluteFill}
      />
      {table.edge ? (
        <LinearGradient
          colors={[table.edge, 'transparent', 'transparent', table.edge]}
          locations={[0, 0.22, 0.78, 1]}
          start={{ x: 0, y: 0.5 }}
          end={{ x: 1, y: 0.5 }}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
    </View>
  );
}
