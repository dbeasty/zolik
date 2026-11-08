import { mutedStyleFor } from '@/src/sound/soundIconStyle';

const AGENTS = {
  mac: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15',
  iphone: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148',
  android: 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36',
  windows: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36',
  linux: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36',
};

describe('mutedStyleFor', () => {
  it('slashes the speaker in the native apps', () => {
    expect(mutedStyleFor('ios')).toBe('slash');
    expect(mutedStyleFor('android')).toBe('slash');
  });

  it('follows the host machine on the web', () => {
    expect(mutedStyleFor('web', AGENTS.mac)).toBe('slash');
    expect(mutedStyleFor('web', AGENTS.iphone)).toBe('slash');
    expect(mutedStyleFor('web', AGENTS.android)).toBe('slash');
    expect(mutedStyleFor('web', AGENTS.windows)).toBe('x');
    expect(mutedStyleFor('web', AGENTS.linux)).toBe('x');
  });

  it('falls back to the X when the host is unknown', () => {
    expect(mutedStyleFor('web')).toBe('x');
    expect(mutedStyleFor('windows')).toBe('x');
  });
});
