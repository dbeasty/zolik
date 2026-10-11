import { narrowOnFirstPaint } from '@/src/hooks/useHydrated';

describe('narrowOnFirstPaint', () => {
  it('is narrow until hydrated, whatever the window is, so the first paint matches the pre-rendered HTML', () => {
    expect(narrowOnFirstPaint(false, false)).toBe(true);
    expect(narrowOnFirstPaint(false, true)).toBe(true);
  });

  it('is the real answer once hydrated', () => {
    expect(narrowOnFirstPaint(true, false)).toBe(false);
    expect(narrowOnFirstPaint(true, true)).toBe(true);
  });
});
