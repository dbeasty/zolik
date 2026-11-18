/**
 * Whether what somebody typed into a username box is really an email
 * address. Accounts made with an address have no password, so typing one on
 * the username screens can only fail to sign in or, worse, make a second
 * account; both screens use this to point at the emailed-code sign-in instead.
 */
export function looksLikeEmail(text: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@.]+$/.test(text.trim());
}
