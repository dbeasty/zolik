import { createElement } from 'react';
import { StyleSheet } from 'react-native';
import { GestureDetector } from 'react-native-gesture-handler';
import { act, create, type ReactTestInstance, type ReactTestRenderer } from 'react-test-renderer';

import { ZoneView } from '@/src/components/match/ZoneView';
import type { Zone } from '@/src/lib/types';
import { handOnDragLayer } from '@/src/theme';

// Reanimated's own mock: `runOnJS(f)` is `f`, so the pan's handlers can be
// called straight from the test, as the gesture would on the JS thread.
jest.mock('react-native-worklets', () => require('react-native-worklets/src/mock'));
jest.mock('react-native-reanimated', () => require('react-native-reanimated/mock'));

/**
 * A card carried off one meld onto another must not change any zIndex on iOS.
 *
 * It did: the card's own wrapper, the meld around it and the hand all took a
 * zIndex the moment a carry began. On iOS a zIndex that changes mid-gesture
 * reorders the native views, which takes the carried card's view out of the
 * window and puts it back — and that cancels the touch carrying it. The card
 * was let go a few frames into the drag, wherever the finger had got to, and
 * a wild could not be dragged from one meld to another. The web draws the
 * same styles without losing the pointer, so the web suite never saw it.
 *
 * jest-expo renders as iOS, which is the platform this is about.
 */

const meld: Zone = {
  id: 'melds:t0',
  kind: 'spread',
  groups: [
    { id: 't0-K', kind: 'set', cards: ['KH', 'KD', 'KS', 'KC', '2C'] },
    { id: 't0-9', kind: 'set', cards: ['9C', '9D', '9S'] },
  ],
} as Zone;

function render(): ReactTestRenderer {
  let tree!: ReactTestRenderer;
  act(() => {
    tree = create(
      createElement(ZoneView, {
        zone: meld,
        pickableGroups: new Set(['t0-K', 't0-9']),
        onMeldDragStart: () => [4],
        onDragMove: () => {},
        onDragEnd: () => true,
      }),
    );
  });
  return tree;
}

const byTestId = (tree: ReactTestRenderer, id: string) =>
  tree.root.find((n) => n.props.testID === id && typeof n.type === 'string');

/** Every zIndex anywhere in the rendered tree, by testID where there is one. */
function zIndexes(tree: ReactTestRenderer): string[] {
  return tree.root
    .findAll((n) => typeof n.type === 'string')
    .map((n, i) => [n.props.testID ?? `#${i}`, StyleSheet.flatten(n.props.style)?.zIndex] as const)
    .filter(([, z]) => z !== undefined)
    .map(([id, z]) => `${id}:${z}`);
}

/** The pan that lifts card `index` of `groupId` off the meld. */
function panOf(tree: ReactTestRenderer, groupId: string, index: number) {
  const card = byTestId(tree, `pick-${groupId}-${index}`);
  let at: ReactTestInstance | null = card;
  while (at && at.type !== GestureDetector) at = at.parent;
  if (!at) throw new Error(`no gesture over pick-${groupId}-${index}`);
  return at.props.gesture.handlers as {
    onStart: (e: object) => void;
    onUpdate: (e: object) => void;
  };
}

describe('carrying a card off a meld on iOS', () => {
  it('leaves every zIndex on the board as it was before the touch', () => {
    const tree = render();
    // Opened first, as a player does: only an open meld's cards can be picked.
    act(() => tree.root.findByProps({ testID: 'group-toggle-t0-K' }).props.onPress());
    const before = zIndexes(tree);

    const pan = panOf(tree, 't0-K', 4);
    act(() => pan.onStart({}));
    act(() => pan.onUpdate({ absoluteX: 40, absoluteY: 300, translationX: -60, translationY: -200 }));

    expect(zIndexes(tree)).toEqual(before);
  });

  it('keeps each card of an open meld in a view of its own from the start', () => {
    // The box a carried card is moved by has no style of its own until the
    // carry gives it a transform. Flattened away until then, iOS created it
    // on the carry's first frame and moved the card into it — the same lost
    // touch as a zIndex change.
    const tree = render();
    act(() => tree.root.findByProps({ testID: 'group-toggle-t0-K' }).props.onPress());
    for (const i of [0, 1, 2, 3, 4]) {
      expect([i, byTestId(tree, `group-card-t0-K-${i}`).props.collapsable]).toEqual([i, false]);
    }
  });
});

describe('the hand on the drag layer', () => {
  it('goes up only for a card that came out of it', () => {
    expect(handOnDragLayer(null)).toBe(false);
    expect(handOnDragLayer({})).toBe(true);
    expect(handOnDragLayer({ fromMeld: 't0-K' })).toBe(false);
    expect(handOnDragLayer({ fromTable: true })).toBe(false);
  });
});
