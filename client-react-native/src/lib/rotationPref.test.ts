jest.mock('expo-screen-orientation', () => ({
  OrientationLock: { DEFAULT: 0, PORTRAIT_UP: 3, LANDSCAPE: 5 },
  lockAsync: jest.fn(() => Promise.resolve()),
}));
jest.mock('@/src/context/SessionContext', () => ({
  storage: { getItem: jest.fn(() => Promise.resolve(null)), setItem: jest.fn(() => Promise.resolve()) },
}));

import { parseRotation, ROTATIONS } from './rotationPref';

describe('parseRotation', () => {
  it('keeps the two locks', () => {
    expect(parseRotation('portrait')).toBe('portrait');
    expect(parseRotation('landscape')).toBe('landscape');
  });

  it('follows the phone when nothing, or nothing it knows, was saved', () => {
    expect(parseRotation(null)).toBe('auto');
    expect(parseRotation(undefined)).toBe('auto');
    expect(parseRotation('sideways')).toBe('auto');
  });

  it('offers following the phone first', () => {
    expect(ROTATIONS[0]).toBe('auto');
  });
});
