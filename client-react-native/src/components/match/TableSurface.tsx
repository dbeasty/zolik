import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Defs, RadialGradient, Rect, Stop } from 'react-native-svg';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';

/**
 * The table itself: whatever the skin says is behind the board.
 *
 * One colour renders flat (the classic dark background). Two or more render
 * as the felt — a vertical wash that runs brighter through the middle band,
 * under a lamp.
 *
 * The lamp used to be four linear gradients darkening from four sides,
 * because a real radial gradient was not among the primitives here. It is
 * now, and one radial gradient is both a better pool of light and less to
 * read than the four washes it replaces. A skin that names no `lamp` keeps
 * the stand-in, which is what leaves `classic` exactly as it was.
 *
 * Absolutely positioned behind everything and `pointerEvents="none"`, so it
 * can never catch a touch or move a measurement.
 */
export function TableSurface() {
  const { table } = useSkin();
  const stops = table.background;
  // The lamp is drawn in real pixels rather than percentages, because an SVG
  // radial gradient's radius is one number and the surface is not square:
  // sized from the box, it stays a circle instead of an ellipse stretched to
  // whatever shape the window happens to be.
  const [size, setSize] = useState({ width: 0, height: 0 });
  const onLayout = (e: LayoutChangeEvent) => setSize(e.nativeEvent.layout);

  if (stops.length < 2) {
    return (
      <View
        pointerEvents="none"
        style={[StyleSheet.absoluteFill, { backgroundColor: stops[0] }]}
      />
    );
  }

  const lamp = table.lamp;

  return (
    <View pointerEvents="none" style={StyleSheet.absoluteFill} onLayout={onLayout}>
      <LinearGradient
        colors={stops as [string, string, ...string[]]}
        style={StyleSheet.absoluteFill}
      />
      {lamp && size.width > 0 ? (
        <Svg width={size.width} height={size.height} style={StyleSheet.absoluteFill}>
          <Defs>
            <RadialGradient
              id="table-lamp"
              cx={size.width * lamp.cx}
              cy={size.height * lamp.cy}
              r={Math.max(size.width, size.height) * lamp.r}
              gradientUnits="userSpaceOnUse"
            >
              <Stop offset="0" stopColor={lamp.inner} />
              <Stop offset="0.55" stopColor="rgba(0, 0, 0, 0)" />
              <Stop offset="1" stopColor={lamp.outer} />
            </RadialGradient>
          </Defs>
          <Rect x="0" y="0" width={size.width} height={size.height} fill="url(#table-lamp)" />
        </Svg>
      ) : null}
      {!lamp && table.edge ? (
        <LinearGradient
          colors={[table.edge, 'transparent', 'transparent', table.edge]}
          locations={[0, 0.22, 0.78, 1]}
          start={{ x: 0, y: 0.5 }}
          end={{ x: 1, y: 0.5 }}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
      {/* The same darkening from the top and bottom edges, so the four
          washes together read as a pool of light rather than a bright
          column — the vertical half of the radial gradient this stands
          in for. */}
      {!lamp && table.edge ? (
        <LinearGradient
          colors={[table.edge, 'transparent', 'transparent', table.edge]}
          locations={[0, 0.18, 0.8, 1]}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
      {/* A slow traverse of the same light across the felt. Twelve seconds
          end to end and barely visible at any instant, which is the point: a
          static felt reads as a picture of a table, and the only thing that
          makes it read as a lit surface is that the light is not quite still.
          Behind everything, catching nothing, and stopped under stillness. */}
      {table.sheen && size.width > 0 ? <Sweep color={table.sheen} width={size.width} /> : null}
      {/* The lamp itself, just off the top-left corner: one diagonal wash
          of light, faint enough to shade the felt rather than sit on it. */}
      {table.sheen ? (
        <LinearGradient
          colors={[table.sheen, 'transparent']}
          start={{ x: 0.1, y: 0 }}
          end={{ x: 0.6, y: 0.75 }}
          style={StyleSheet.absoluteFill}
        />
      ) : null}
    </View>
  );
}

/**
 * The traverse. One wide, very faint band drifting across and back, on the
 * native driver and on transform alone — it is behind the whole board, so it
 * must cost nothing per frame and move nothing that is measured.
 */
function Sweep({ color, width }: { color: string; width: number }) {
  const progress = useRef(new Animated.Value(0)).current;
  const stillness = useReducedMotion();

  useEffect(() => {
    if (stillness) return;
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(progress, {
          toValue: 1,
          duration: 12000,
          easing: Easing.inOut(Easing.quad),
          useNativeDriver: true,
        }),
        Animated.timing(progress, {
          toValue: 0,
          duration: 12000,
          easing: Easing.inOut(Easing.quad),
          useNativeDriver: true,
        }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [stillness, progress]);

  if (stillness) return null;

  const band = Math.max(180, width * 0.45);
  return (
    <Animated.View
      pointerEvents="none"
      style={{
        position: 'absolute',
        top: 0,
        bottom: 0,
        left: -band,
        width: band,
        transform: [
          {
            translateX: progress.interpolate({
              inputRange: [0, 1],
              outputRange: [0, width + band],
            }),
          },
        ],
      }}
    >
      <LinearGradient
        colors={['transparent', color, 'transparent']}
        start={{ x: 0, y: 0 }}
        end={{ x: 1, y: 0 }}
        style={StyleSheet.absoluteFill}
      />
    </Animated.View>
  );
}
