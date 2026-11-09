jest.mock('expo-modules-core', () => ({
  requireOptionalNativeModule: () => ({
    bleHostStart: jest.fn(async () => {}),
    bleHostStop: jest.fn(async () => {}),
  }),
}));

import { bleHostStart, bleHostStop, bleIsHosting } from '../../modules/zolik-nearby';

describe('bleIsHosting', () => {
  it('follows advertising, whichever screen started or stopped it', async () => {
    expect(bleIsHosting()).toBe(false);
    await bleHostStart('Table');
    expect(bleIsHosting()).toBe(true);
    await bleHostStop();
    expect(bleIsHosting()).toBe(false);
  });
});
