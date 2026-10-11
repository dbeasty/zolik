import { Platform, StyleSheet, type ViewStyle } from 'react-native';

/**
 * A looping decoration drawn by the browser's own animation engine.
 *
 * React Native's `Animated` has no native driver on the web: every looping
 * pulse, sweep and drifting glyph falls back to JavaScript and a frame request
 * sixty times a second, each one a style write the page then re-renders. In
 * the Mac app that kept a window with nothing happening in it at 15-25% of a
 * core. A CSS animation of transform and opacity runs off the main thread and
 * costs nothing while the page is idle.
 *
 * Web only: off the web this returns undefined and the caller keeps its
 * `Animated` loop. Keyframes are keyed by their content, so the same loop
 * asked for twice shares one rule.
 */

export type LoopFrames = Record<string, ViewStyle>;

export type LoopOptions = {
  /** One pass, in milliseconds (already scaled by `ms`). */
  duration: number;
  /** 'alternate' plays forward then back, as a 0→1→0 `Animated.sequence` did. */
  direction?: 'normal' | 'alternate';
  /** A CSS timing function. */
  timing?: string;
  /** A one-off head start before the first pass. */
  delay?: number;
};

const cache = new Map<string, ViewStyle>();

type Transform = Record<string, number | string>;

/**
 * react-native-web writes a keyframe's `transform` array out as
 * "[object Object]", which the browser drops: inside keyframes the transform
 * has to be the CSS string itself.
 */
function transformString(list: Transform[]): string {
  return list
    .map((t) => {
      const [name, value] = Object.entries(t)[0] as [string, number | string];
      const px = typeof value === 'number' && /^translate/.test(name) ? `${value}px` : value;
      return `${name}(${px})`;
    })
    .join(' ');
}

export function cssFrames(frames: LoopFrames): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [stop, style] of Object.entries(frames)) {
    const copy: Record<string, unknown> = { ...style };
    if (Array.isArray(copy.transform)) copy.transform = transformString(copy.transform as Transform[]);
    out[stop] = copy;
  }
  return out;
}

export function cssLoop(frames: LoopFrames, opts: LoopOptions): ViewStyle | undefined {
  if (Platform.OS !== 'web') return undefined;
  const key = JSON.stringify([frames, opts]);
  let style = cache.get(key);
  if (!style) {
    style = StyleSheet.create({
      loop: {
        animationKeyframes: cssFrames(frames),
        animationDuration: `${Math.round(opts.duration)}ms`,
        animationIterationCount: 'infinite',
        animationDirection: opts.direction ?? 'normal',
        animationTimingFunction: opts.timing ?? 'linear',
        animationDelay: `${Math.round(opts.delay ?? 0)}ms`,
        // Before the first pass the element already stands at the first frame.
        animationFillMode: 'backwards',
      } as unknown as ViewStyle,
    }).loop;
    cache.set(key, style);
  }
  return style;
}
