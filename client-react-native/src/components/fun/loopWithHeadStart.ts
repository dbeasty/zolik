import { Animated } from 'react-native';

/**
 * Runs `config` over and over on `value`, 0 → 1, after a single head start.
 *
 * `Animated.loop` cannot say "wait once, then repeat": a `delay` inside the
 * timing is paid again on every pass, so a staggered row of dancers drifts out
 * of rhythm with itself. Restarting by hand is five lines and keeps the beat.
 *
 * Returns the stop function an effect hands back as its cleanup.
 */
export function loopWithHeadStart(
  value: Animated.Value,
  config: Omit<Animated.TimingAnimationConfig, 'toValue' | 'delay'>,
  headStartMs = 0,
): () => void {
  let alive = true;
  let current: Animated.CompositeAnimation | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const run = () => {
    if (!alive) return;
    value.setValue(0);
    current = Animated.timing(value, { ...config, toValue: 1 });
    current.start(({ finished }) => {
      if (finished) run();
    });
  };

  if (headStartMs > 0) timer = setTimeout(run, headStartMs);
  else run();

  return () => {
    alive = false;
    if (timer) clearTimeout(timer);
    current?.stop();
  };
}
