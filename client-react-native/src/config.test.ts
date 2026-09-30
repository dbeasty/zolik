// config.ts computes CLIENT_VERSION/CLIENT_COMMIT at module scope, from
// process.env.EXPO_PUBLIC_*, so each case here resets the module registry
// and re-requires it rather than importing once at the top of the file.

const savedEnv = { ...process.env };

afterEach(() => {
  process.env = { ...savedEnv };
  jest.resetModules();
});

it('reports the build the bundler was given', () => {
  process.env.EXPO_PUBLIC_ZOLIK_VERSION = '1.1.1.2';
  process.env.EXPO_PUBLIC_ZOLIK_COMMIT = '7feb025';
  jest.resetModules();

  const config = require('./config');

  expect(config.CLIENT_VERSION).toBe('1.1.1.2');
  expect(config.CLIENT_COMMIT).toBe('7feb025');
});

it('falls back to a build that is obviously not a real release', () => {
  delete process.env.EXPO_PUBLIC_ZOLIK_VERSION;
  delete process.env.EXPO_PUBLIC_ZOLIK_COMMIT;
  jest.resetModules();

  const config = require('./config');

  expect(config.CLIENT_VERSION).toBe('0.0.0-dev');
  expect(config.CLIENT_COMMIT).toBe('unknown');
});

it('reports the operator the deploy script named', () => {
  process.env.EXPO_PUBLIC_ZOLIK_OPERATOR = 'Limidus Corp';
  process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_COUNTRY = 'Czechia';
  process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_CONTACT = 'legal@limidus.com';
  jest.resetModules();

  const config = require('./config');

  expect(config.OPERATOR_NAME).toBe('Limidus Corp');
  expect(config.OPERATOR_COUNTRY).toBe('Czechia');
  expect(config.OPERATOR_CONTACT).toBe('legal@limidus.com');
});

it('leaves the operator empty rather than inventing one', () => {
  // Empty is what `src/legal` reads as "not named yet" and answers with a
  // draft banner. A default here would defeat that by naming somebody.
  delete process.env.EXPO_PUBLIC_ZOLIK_OPERATOR;
  delete process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_COUNTRY;
  delete process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_CONTACT;
  jest.resetModules();

  const config = require('./config');

  expect(config.OPERATOR_NAME).toBe('');
  expect(config.OPERATOR_COUNTRY).toBe('');
  expect(config.OPERATOR_CONTACT).toBe('');
});

describe('ZOLIK_BASE_URL', () => {
  // The production image serves the web bundle from the API server itself, on
  // several domains at once, so a browser must call whichever one served it.
  function loadAs(os: string, origin: string | undefined) {
    jest.resetModules();
    jest.doMock('react-native', () => ({ Platform: { OS: os } }));
    jest.doMock('expo-constants', () => ({ __esModule: true, default: {} }));
    const g = globalThis as { window?: unknown };
    const had = 'window' in g;
    const saved = g.window;
    g.window = origin === undefined ? undefined : { location: { origin } };
    try {
      return require('./config').ZOLIK_BASE_URL as string;
    } finally {
      if (had) g.window = saved;
      else delete g.window;
      jest.dontMock('react-native');
      jest.dontMock('expo-constants');
    }
  }

  it('is the serving origin on web when the build says the API is same-origin', () => {
    process.env.EXPO_PUBLIC_ZOLIK_BASE_URL = 'https://jokerless.com';
    process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN = '1';

    expect(loadAs('web', 'https://jokerless.org')).toBe('https://jokerless.org');
    expect(loadAs('web', 'https://play.limidus.com')).toBe('https://play.limidus.com');
  });

  it('keeps the baked-in URL during the static prerender, where there is no window', () => {
    process.env.EXPO_PUBLIC_ZOLIK_BASE_URL = 'https://jokerless.com/';
    process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN = '1';

    expect(loadAs('web', undefined)).toBe('https://jokerless.com');
  });

  it('keeps the baked-in URL on native, whatever the flag says', () => {
    process.env.EXPO_PUBLIC_ZOLIK_BASE_URL = 'https://jokerless.com';
    process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN = '1';

    expect(loadAs('ios', 'https://jokerless.org')).toBe('https://jokerless.com');
  });

  it('keeps the baked-in URL on web without the flag, as under the Expo dev server', () => {
    // The client is on :8114 there and the API on :8090 — different origins.
    process.env.EXPO_PUBLIC_ZOLIK_BASE_URL = 'http://127.0.0.1:8090';
    delete process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN;

    expect(loadAs('web', 'http://127.0.0.1:8114')).toBe('http://127.0.0.1:8090');
  });
});
