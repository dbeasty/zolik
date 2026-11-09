import { requestAgeStatus } from './requestAge.native';

const mockRequireOptional = jest.fn();
jest.mock('expo', () => ({
  requireOptionalNativeModule: (name: string) => mockRequireOptional(name),
}));

function nativeAnswering(answer: () => Promise<unknown>) {
  const native = { requestAgeRangeAsync: jest.fn(answer) };
  mockRequireOptional.mockReturnValue(native);
  return native;
}

beforeEach(() => mockRequireOptional.mockReset());

describe('requestAgeStatus without the native module', () => {
  it('has no age signal: neither locks the chip games nor calls the player an adult', async () => {
    mockRequireOptional.mockReturnValue(null);
    await expect(requestAgeStatus()).resolves.toBe('unknown');
  });
});

describe('requestAgeStatus with the native module', () => {
  it('asks about 18', async () => {
    const native = nativeAnswering(async () => ({ lowerBound: 18, upperBound: null }));
    await requestAgeStatus();
    expect(native.requestAgeRangeAsync).toHaveBeenCalledWith({ threshold1: 18 });
  });
  it('reads the answer', async () => {
    nativeAnswering(async () => ({ lowerBound: 18, upperBound: null }));
    await expect(requestAgeStatus()).resolves.toBe('adult');
    nativeAnswering(async () => ({ lowerBound: 13, upperBound: 17 }));
    await expect(requestAgeStatus()).resolves.toBe('minor');
  });
  it('reads a refusal to share as under 18, and any other failure as no signal', async () => {
    nativeAnswering(() => Promise.reject({ code: 'ERR_AGE_RANGE_USER_DECLINED' }));
    await expect(requestAgeStatus()).resolves.toBe('minor');
    nativeAnswering(() => Promise.reject({ code: 'ERR_AGE_RANGE_NOT_AVAILABLE' }));
    await expect(requestAgeStatus()).resolves.toBe('unknown');
  });
});

// A rename upstream would otherwise read as "module absent" in a store build:
// no error anywhere, and the under-18 gate silently off.
describe('the installed expo-age-range', () => {
  it('registers the native module under the name asked for, and only forwards to it', async () => {
    const asked: string[] = [];
    const native = { requestAgeRangeAsync: jest.fn(async () => ({ lowerBound: 18, upperBound: null })) };
    jest.doMock('expo-modules-core', () => ({
      requireNativeModule: (name: string) => {
        asked.push(name);
        return native;
      },
    }));
    const pkg = jest.requireActual<typeof import('expo-age-range')>('expo-age-range');
    const options = { threshold1: 18 };
    await expect(pkg.requestAgeRangeAsync(options)).resolves.toEqual({ lowerBound: 18, upperBound: null });
    expect(native.requestAgeRangeAsync).toHaveBeenCalledWith(options);

    mockRequireOptional.mockReturnValue(null);
    await requestAgeStatus();
    expect(asked).toEqual([mockRequireOptional.mock.calls[0][0]]);
  });
});
