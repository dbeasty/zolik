/**
 * WCAG 2.x contrast, for the skin contrast test (`contrast.test.ts`).
 *
 * Small on purpose: the formula is the standard's own — relative luminance
 * from linearised sRGB, then (L1 + 0.05) / (L2 + 0.05) — and the only thing
 * added is compositing, because half the palettes here are translucent glass
 * over a felt, and a translucent colour has no contrast until you say what
 * is under it.
 */

export type Rgba = [number, number, number, number];

/** `#rrggbb`, `#rgb`, `rgb(…)` or `rgba(…)`. Throws on anything else, so a typo fails the test loudly. */
export function parseColor(c: string): Rgba {
  const s = c.trim();
  let m = s.match(/^#([0-9a-f]{6})$/i);
  if (m) {
    const n = parseInt(m[1]!, 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255, 1];
  }
  m = s.match(/^#([0-9a-f]{3})$/i);
  if (m) {
    const [r, g, b] = m[1]!.split('').map((h) => parseInt(h + h, 16));
    return [r!, g!, b!, 1];
  }
  m = s.match(/^rgba?\(([^)]+)\)$/i);
  if (m) {
    const p = m[1]!.split(',').map((x) => Number(x.trim()));
    if (p.length >= 3 && p.every((x) => Number.isFinite(x))) return [p[0]!, p[1]!, p[2]!, p[3] ?? 1];
  }
  throw new Error(`not a colour: ${c}`);
}

/** `top` painted over an opaque `under`. */
export function over(top: Rgba, under: Rgba): Rgba {
  const a = top[3];
  return [
    top[0] * a + under[0] * (1 - a),
    top[1] * a + under[1] * (1 - a),
    top[2] * a + under[2] * (1 - a),
    1,
  ];
}

/**
 * Flattens a stack of colours, topmost first, onto the last (which must be
 * opaque, or is treated as if over black).
 */
export function flatten(...layers: string[]): Rgba {
  let acc = over(parseColor(layers[layers.length - 1]!), [0, 0, 0, 1]);
  for (let i = layers.length - 2; i >= 0; i--) acc = over(parseColor(layers[i]!), acc);
  return acc;
}

export function luminance([r, g, b]: Rgba): number {
  const lin = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

export function ratio(a: Rgba, b: Rgba): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Contrast of `fg` (possibly translucent) on the background stack `bg…`, topmost first. */
export function contrast(fg: string, ...bg: string[]): number {
  const back = flatten(...bg);
  return ratio(over(parseColor(fg), back), back);
}
