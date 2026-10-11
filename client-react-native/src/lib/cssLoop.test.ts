import { cssFrames, cssLoop } from '@/src/lib/cssLoop';

describe('cssLoop', () => {
  it('writes a keyframe transform as the CSS string the browser reads', () => {
    expect(
      cssFrames({
        '0%': { opacity: 0.5, transform: [{ translateX: 0 }, { rotateZ: '-12deg' }, { scale: 1.2 }] },
      }),
    ).toEqual({ '0%': { opacity: 0.5, transform: 'translateX(0px) rotateZ(-12deg) scale(1.2)' } });
  });

  it('is nothing off the web, where the caller keeps its Animated loop', () => {
    expect(cssLoop({ '0%': { opacity: 0 }, '100%': { opacity: 1 } }, { duration: 100 })).toBeUndefined();
  });
});
