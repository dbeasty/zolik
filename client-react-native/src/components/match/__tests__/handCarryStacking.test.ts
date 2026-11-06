import { createElement } from 'react';
import { StyleSheet } from 'react-native';
import { GestureDetector } from 'react-native-gesture-handler';
import { act, create, type ReactTestInstance, type ReactTestRenderer } from 'react-test-renderer';

import { HandZone } from '@/src/components/match/HandZone';
import type { Zone } from '@/src/lib/types';

// Reanimated's own mock: `runOnJS(f)` is `f`, so the pan's handlers can be
// called straight from the test, as the gesture would on the JS thread.
jest.mock('react-native-worklets', () => require('react-native-worklets/src/mock'));
jest.mock('react-native-reanimated', () => require('react-native-reanimated/mock'));

/**
 * A card dragged along the hand must not change any zIndex on iOS.
 *
 * It did: the hand's panel and its row went up onto the drag layer the moment
 * a card lifted. On iOS a zIndex that changes mid-gesture reorders the native
 * views, and that cancels the touch carrying the card — every card dragged
 * along the fan was let go two moves after it lifted, wherever the finger had
 * got to, so a hand could only be rearranged a card's width at a time. The
 * web draws the same styles without losing the pointer, so the web suite
 * never saw it. See also `meldCarryStacking.test.ts`, the same bug off a meld.
 *
 * The card's own lift (`styles.lifted`) is left out of the comparison: it
 * ranks the card among its siblings in the row, and on the simulator drags
 * ended cleanly with it in place.
 *
 * jest-expo renders as iOS, which is the platform this is about.
 */

const zone = { id: 'hand:me', kind: 'hand', count: 5 } as unknown as Zone;
const slots = ['3S', '4S', '5S', 'JC', 'KH'].map((card, i) => ({ id: `s${i}`, card }));

function render(): ReactTestRenderer {
  let tree!: ReactTestRenderer;
  act(() => {
    tree = create(
      createElement(HandZone, {
        zone,
        slots,
        selected: new Set<string>(),
        onToggle: () => {},
        onMove: () => {},
        onDragMove: () => {},
        onDragEnd: () => true,
      } as never),
    );
  });
  return tree;
}

const byTestId = (tree: ReactTestRenderer, id: string) =>
  tree.root.find((n) => n.props.testID === id && typeof n.type === 'string');

/** Every zIndex outside the cards themselves, by testID where there is one. */
function zIndexes(tree: ReactTestRenderer): string[] {
  const cards = new Set(
    tree.root
      .findAll((n) => n.type === GestureDetector)
      .flatMap((d) => d.findAll((n) => typeof n.type === 'string')),
  );
  return tree.root
    .findAll((n) => typeof n.type === 'string' && !cards.has(n))
    .map((n, i) => [n.props.testID ?? `#${i}`, StyleSheet.flatten(n.props.style)?.zIndex] as const)
    .filter(([, z]) => z !== undefined)
    .map(([id, z]) => `${id}:${z}`);
}

/** The pan that lifts card `index` out of the hand. */
function panOf(tree: ReactTestRenderer, index: number) {
  let at: ReactTestInstance | null = byTestId(tree, `card-${zone.id}-${index}`);
  while (at && at.type !== GestureDetector) at = at.parent;
  if (!at) throw new Error(`no gesture over card ${index}`);
  const gesture = at.props.gesture;
  // Composed with the badge long-press: the pan is the one with `onUpdate`.
  const pan = (gesture.gestures ?? [gesture]).find(
    (g: { handlers: { onUpdate?: unknown } }) => g.handlers.onUpdate,
  );
  return pan.handlers as {
    onBegin: (e: object) => void;
    onStart: (e: object) => void;
    onUpdate: (e: object) => void;
  };
}

describe('dragging a card along the hand on iOS', () => {
  it('leaves every zIndex around the cards as it was before the touch', () => {
    const tree = render();
    const before = zIndexes(tree);
    // Raised from the start, so a card carried out over the board is drawn
    // above it without anything changing once it has lifted.
    expect(before).toContain(`hand-${zone.id}:100`);

    const pan = panOf(tree, 0);
    act(() => pan.onBegin({ absoluteX: 20, absoluteY: 600 }));
    act(() => pan.onStart({}));
    act(() => pan.onUpdate({ absoluteX: 120, absoluteY: 600, translationX: 100, translationY: 0 }));

    expect(zIndexes(tree)).toEqual(before);
  });
});
