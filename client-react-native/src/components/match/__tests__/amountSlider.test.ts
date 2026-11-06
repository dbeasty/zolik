import { createElement } from 'react';
import { GestureDetector } from 'react-native-gesture-handler';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { AmountSlider, type OfferBarStyles } from '@/src/components/match/OfferBar';

jest.mock('react-native-worklets', () => require('react-native-worklets/src/mock'));
jest.mock('react-native-reanimated', () => require('react-native-reanimated/mock'));

/**
 * The stake slider (Blackjack's bet, Hold'em's "Raise to") lives inside the
 * match screen's ScrollView. On iOS it was a plain JS responder, and the
 * scroll view's native pan took every sideways drag from it: the page
 * scrolled and the stake stayed put. It is now a gesture-handler pan, a
 * native recognizer that claims a sideways drag before the scroll view does
 * and gives up a vertical one so the page still scrolls.
 *
 * jest-expo renders as iOS, which is the platform this is about.
 */

type Handlers = { onStart?: (e: object) => void; onUpdate?: (e: object) => void; onEnd?: (e: object) => void };
type Composed = { gestures: { handlers: Handlers; config: Record<string, unknown> }[] };

function render(onChange: (n: number) => void): ReactTestRenderer {
  let tree!: ReactTestRenderer;
  act(() => {
    tree = create(
      createElement(AmountSlider, {
        name: 'Stake',
        min: 10,
        max: 110,
        step: 10,
        value: 10,
        onChange,
        styles: {} as OfferBarStyles,
        testID: 'param-stake-slider',
      }),
    );
  });
  // A 118-point track: an 18-point thumb whose centre runs over 100 points,
  // one point per unit of stake.
  const track = tree.root.find((n) => n.props.testID === 'param-stake-slider' && typeof n.type === 'string');
  act(() => track.props.onLayout({ nativeEvent: { layout: { width: 118, height: 28, x: 0, y: 0 } } }));
  return tree;
}

function gestureOf(tree: ReactTestRenderer): Composed {
  return tree.root.findByType(GestureDetector).props.gesture as Composed;
}

describe('the stake slider on iOS', () => {
  it('is driven by a native gesture, not a JS responder the scroll view can cancel', () => {
    const tree = render(() => {});
    const track = tree.root.find((n) => n.props.testID === 'param-stake-slider' && typeof n.type === 'string');
    expect(track.props.onResponderGrant).toBeUndefined();
    expect(track.props.onStartShouldSetResponder).toBeUndefined();
    expect(gestureOf(tree).gestures).toHaveLength(2);
  });

  it('claims a sideways drag and leaves a vertical one to the page', () => {
    const pan = gestureOf(render(() => {})).gestures[0];
    // Activates on a few points sideways, fails on a few up or down — both
    // under the ~10 points a UIScrollView needs before it begins.
    expect(pan.config.activeOffsetXStart).toBeLessThan(0);
    expect(pan.config.activeOffsetXStart).toBeGreaterThanOrEqual(-8);
    expect(pan.config.activeOffsetXEnd).toBeGreaterThan(0);
    expect(pan.config.activeOffsetXEnd).toBeLessThanOrEqual(8);
    expect(pan.config.failOffsetYStart).toBeGreaterThanOrEqual(-9);
    expect(pan.config.failOffsetYEnd).toBeLessThanOrEqual(9);
  });

  it('moves the stake with the finger, snapped to the step and clamped to the range', () => {
    const seen: number[] = [];
    const [pan, tap] = gestureOf(render((n) => seen.push(n))).gestures;
    act(() => pan.handlers.onStart?.({ x: 9 }));
    act(() => pan.handlers.onUpdate?.({ x: 59 }));
    act(() => pan.handlers.onUpdate?.({ x: 63 }));
    act(() => pan.handlers.onUpdate?.({ x: 500 }));
    act(() => pan.handlers.onUpdate?.({ x: -40 }));
    act(() => tap.handlers.onEnd?.({ x: 89 }));
    expect(seen).toEqual([10, 60, 60, 110, 10, 90]);
  });
});
