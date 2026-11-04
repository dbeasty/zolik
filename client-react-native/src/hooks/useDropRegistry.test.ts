import React from 'react';
import { act, create } from 'react-test-renderer';

import { intersect, useDropRegistry, type Measurable } from '@/src/hooks/useDropRegistry';

type Registry = ReturnType<typeof useDropRegistry>;

function at(x: number, y: number, width: number, height: number): Measurable {
  return { measureInWindow: (cb) => cb(x, y, width, height) };
}

function mount(): Registry {
  let registry: Registry | null = null;
  function Probe() {
    registry = useDropRegistry();
    return null;
  }
  act(() => {
    create(React.createElement(Probe));
  });
  return registry!;
}

describe('intersect', () => {
  it('is the overlap, or nothing', () => {
    expect(intersect({ x: 0, y: 0, width: 100, height: 100 }, { x: 50, y: 50, width: 100, height: 100 })).toEqual({
      x: 50,
      y: 50,
      width: 50,
      height: 50,
    });
    expect(intersect({ x: 0, y: 0, width: 10, height: 10 }, { x: 20, y: 20, width: 5, height: 5 })).toBeNull();
  });
});

describe('useDropRegistry with the hand docked above the board', () => {
  // The board scrolls in the window from y=300 down; the dock fills 0-300.
  const board = at(0, 300, 800, 500);

  it('refuses a meld scrolled up under the dock', () => {
    const r = mount();
    r.setViewport(board);
    r.register('group-hidden', at(100, 120, 120, 100));
    r.measure();
    expect(r.hit(150, 170, ['group-hidden'])).toBeNull();
  });

  it('takes a meld half under the dock only on its visible half', () => {
    const r = mount();
    r.setViewport(board);
    r.register('group-half', at(100, 250, 120, 120));
    r.measure();
    expect(r.hit(150, 340, ['group-half'])).toBe('group-half');
    // Over the dock, where the meld's top half is hidden.
    expect(r.hit(150, 270, ['group-half'])).toBeNull();
  });

  it('still prefers a meld over the section around it when the section is half hidden', () => {
    const r = mount();
    r.setViewport(board);
    // The section runs far up under the dock, so its visible part is smaller
    // than the meld inside it; the meld is still the smaller region.
    r.register('zone-melds', at(0, -400, 800, 760));
    r.register('group-m', at(100, 310, 200, 200));
    r.measure();
    expect(r.hit(150, 340, ['zone-melds', 'group-m'])).toBe('group-m');
  });

  it('still takes a drop on the docked hand itself', () => {
    const r = mount();
    r.setViewport(board);
    r.registerUnclipped('zone-hand:me', at(0, 100, 800, 150));
    r.measure();
    expect(r.hit(400, 180, ['zone-hand:me'])).toBe('zone-hand:me');
  });

  it('keeps the whole rect for placing a card along a meld', () => {
    const r = mount();
    r.setViewport(board);
    r.register('group-half', at(100, 250, 120, 120));
    r.measure();
    expect(r.rectFor('group-half')).toEqual({ x: 100, y: 250, width: 120, height: 120 });
    expect(r.viewport()).toEqual({ x: 0, y: 300, width: 800, height: 500 });
  });
});
