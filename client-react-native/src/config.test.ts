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
