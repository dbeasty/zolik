import { looksLikeEmail } from './looksLikeEmail';

describe('looksLikeEmail', () => {
  it('knows an address from a username', () => {
    expect(looksLikeEmail('msft_davja@hotmail.com')).toBe(true);
    expect(looksLikeEmail('  Someone@Example.co.uk ')).toBe(true);
    expect(looksLikeEmail('msft_davja')).toBe(false);
    expect(looksLikeEmail('ada@home')).toBe(false);
    expect(looksLikeEmail('two words@example.com')).toBe(false);
  });
});
