/**
 * Stack screens stay mounted under pushed screens, so the match board's
 * document key listener must go quiet while Rules or Settings is on top —
 * otherwise Enter/D/T/U play moves from a screen that shows none of it.
 */
import React, { useRef } from 'react';
import { Platform } from 'react-native';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

let mockFocused = true;
jest.mock('expo-router', () => ({ useIsFocused: () => mockFocused }));

import { useFocusedKeydown } from '@/src/hooks/useFocusedKeydown';

function Board({ onKey }: { onKey: (e: KeyboardEvent) => void }) {
  const ref = useRef(onKey);
  ref.current = onKey;
  useFocusedKeydown(ref);
  return null;
}

function press(key: string) {
  const e = new Event('keydown') as Event & { key: string };
  e.key = key;
  document.dispatchEvent(e);
}

describe('useFocusedKeydown', () => {
  const g = global as { document?: unknown };
  const realDocument = g.document;
  let renderer: ReactTestRenderer | null = null;

  beforeEach(() => {
    mockFocused = true;
    g.document = new EventTarget();
    jest.replaceProperty(Platform, 'OS', 'web');
  });

  afterEach(() => {
    if (renderer) act(() => renderer!.unmount());
    renderer = null;
    g.document = realDocument;
    jest.restoreAllMocks();
  });

  function render(onKey: jest.Mock) {
    act(() => {
      renderer = create(React.createElement(Board, { onKey }));
    });
    return () => act(() => renderer!.update(React.createElement(Board, { onKey })));
  }

  it('handles keys while the screen is focused', () => {
    const onKey = jest.fn();
    render(onKey);
    press('Enter');
    expect(onKey).toHaveBeenCalledTimes(1);
  });

  it('ignores keys while another screen is pushed on top, and resumes on return', () => {
    const onKey = jest.fn();
    const rerender = render(onKey);

    mockFocused = false; // Rules pushed over the match
    rerender();
    press('Enter');
    press('d');
    press('t');
    press('u');
    expect(onKey).not.toHaveBeenCalled();

    mockFocused = true; // back to the match
    rerender();
    press('Enter');
    expect(onKey).toHaveBeenCalledTimes(1);
  });

  it('stops listening once unmounted', () => {
    const onKey = jest.fn();
    render(onKey);
    act(() => renderer!.unmount());
    renderer = null;
    press('Enter');
    expect(onKey).not.toHaveBeenCalled();
  });
});
