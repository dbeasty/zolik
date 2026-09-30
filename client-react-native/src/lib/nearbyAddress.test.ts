import { nearbyBaseUrl } from '@/src/lib/nearbyAddress';

describe('nearbyBaseUrl', () => {
  it.each([
    ['192.168.1.20:47800', 'http://192.168.1.20:47800'],
    [' 192.168.1.20:47800/ ', 'http://192.168.1.20:47800'],
    ['http://10.0.2.2:47800', 'http://10.0.2.2:47800'],
    ['HTTP://10.0.2.2:47800//', 'HTTP://10.0.2.2:47800'],
  ])('%j -> %s', (typed, want) => {
    expect(nearbyBaseUrl(typed)).toBe(want);
  });
});
