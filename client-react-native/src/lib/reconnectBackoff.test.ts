import { busyBackoff, jitteredBackoff } from '@/src/lib/reconnectBackoff';

describe('jitteredBackoff', () => {
  afterEach(() => jest.restoreAllMocks());

  it('doubles per attempt from the base, with no jitter at the low end of the draw', () => {
    jest.spyOn(Math, 'random').mockReturnValue(0);
    expect(jitteredBackoff(0)).toBe(1000);
    expect(jitteredBackoff(1)).toBe(2000);
    expect(jitteredBackoff(2)).toBe(4000);
  });

  it('never exceeds the cap by more than the 25% jitter', () => {
    jest.spyOn(Math, 'random').mockReturnValue(0.999999);
    for (const attempt of [4, 10, 50]) {
      const ms = jitteredBackoff(attempt);
      expect(ms).toBeGreaterThanOrEqual(10_000);
      expect(ms).toBeLessThanOrEqual(12_500);
    }
  });

  it('spreads a crowd out: two clients on the same attempt do not wait the same time', () => {
    const draws = [0.1, 0.9];
    jest.spyOn(Math, 'random').mockImplementation(() => draws.shift() ?? 0);
    expect(jitteredBackoff(2)).not.toBe(jitteredBackoff(2));
  });
});

describe('busyBackoff', () => {
  it('waits far longer than an ordinary reconnect when the server says it is full', () => {
    jest.spyOn(Math, 'random').mockReturnValue(0);
    expect(busyBackoff(0)).toBe(30_000);
    expect(busyBackoff(1)).toBe(60_000);
    expect(busyBackoff(9)).toBe(120_000);
    jest.restoreAllMocks();
  });
});
