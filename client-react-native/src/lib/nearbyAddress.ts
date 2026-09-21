/**
 * The http base for a nearby table's address, however the player came by
 * it: typed ("192.168.1.20:47800"), scanned from a host's QR code, or handed
 * over by discovery. Anything already carrying a scheme is kept as it is.
 * Surrounding space and trailing slashes are dropped, because they are what
 * a typed or pasted address picks up.
 */
export function nearbyBaseUrl(address: string): string {
  const a = address.trim().replace(/\/+$/, '');
  return /^https?:\/\//i.test(a) ? a : `http://${a}`;
}
