/**
 * The Last Card pack, as drawing data.
 *
 * An original deck: four colours that are not the familiar commercial four
 * (coral, teal, violet and amber), each paired with a shape of its own so a
 * colour can be told apart by a player who does not see it, and each with a
 * picture behind its cards — a sunburst, the sea, a night sky over hills, a
 * honeycomb. The wilds are dusk-dark, with the four colours turning round the
 * middle.
 *
 * Everything here is geometry in a field 100 units wide and `h` tall, so it is
 * testable without a renderer and scales to any card. `LastCardFace` puts it
 * on screen. The card's colours are the pack's own and no skin changes them:
 * a skin may restyle the stock and the edge, never the cards' colours
 * (`src/skins/types.ts`).
 */

export type Colour = 'C' | 'T' | 'V' | 'A';
export const COLOURS: readonly Colour[] = ['C', 'T', 'V', 'A'];

/** One colour's inks, darkest to palest. */
export type Inks = { deep: string; main: string; tint: string; pale: string };

export const INKS: Record<Colour, Inks> = {
  C: { deep: '#9F3A1F', main: '#E8603C', tint: '#F6B49B', pale: '#FDEBE3' },
  T: { deep: '#0B5E57', main: '#14A193', tint: '#8FDCD1', pale: '#E2F6F3' },
  V: { deep: '#46309A', main: '#7A5BD8', tint: '#C4B5F4', pale: '#EFEAFD' },
  A: { deep: '#8C5A06', main: '#E5A019', tint: '#F6D58C', pale: '#FDF4DD' },
};

/** The wilds' own field and the cream every glyph is printed in. */
export const DUSK = { field: '#1F1C2B', glow: '#3A3352', star: '#F4EBD0' };
export const CREAM = '#FFF8EA';

/**
 * The face's type: rounded and heavy where the platform has such a face,
 * falling back to the system sans. An SVG's text does not inherit the app's
 * font, and left alone it is set in the browser's serif.
 */
export const LAST_CARD_FONT =
  "ui-rounded, 'SF Pro Rounded', 'Arial Rounded MT Bold', 'Nunito', system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif";

/** Which shape stands for which colour. */
export const SHAPE_OF: Record<Colour, 'circle' | 'diamond' | 'triangle' | 'square'> = {
  C: 'circle',
  T: 'diamond',
  V: 'triangle',
  A: 'square',
};

/** The shape glyph for a colour, for text: a card named inside a sentence. */
export const SHAPE_GLYPH: Record<Colour, string> = { C: '●', T: '◆', V: '▲', A: '■' };

/** One drawn piece. `fill`/`stroke` absent means none. */
export type Piece = {
  d: string;
  fill?: string;
  stroke?: string;
  sw?: number;
  opacity?: number;
};

/** A parsed Last Card code. */
export type LastCard =
  | { kind: 'number'; colour: Colour; face: string }
  | { kind: 'skip' | 'reverse' | 'drawTwo'; colour: Colour; face: string }
  | { kind: 'wild' | 'wildDrawFour'; colour: null; face: string };

const CODE = /^([CTVA])-([0-9]|S|R|D)$/;

/** Whether a string is one of this pack's codes: "C-7", "T-S", "W", "W4". */
export function isLastCardCode(value: string): boolean {
  return CODE.test(value) || value === 'W' || value === 'W4';
}

export function parseLastCard(code: string): LastCard | null {
  if (code === 'W') return { kind: 'wild', colour: null, face: 'W' };
  if (code === 'W4') return { kind: 'wildDrawFour', colour: null, face: '+4' };
  const m = CODE.exec(code);
  if (!m) return null;
  const colour = m[1] as Colour;
  switch (m[2]) {
    case 'S':
      return { kind: 'skip', colour, face: 'S' };
    case 'R':
      return { kind: 'reverse', colour, face: 'R' };
    case 'D':
      return { kind: 'drawTwo', colour, face: '+2' };
  }
  return { kind: 'number', colour, face: m[2] };
}

// --- shapes ------------------------------------------------------------------

const f = (n: number) => (Math.round(n * 100) / 100).toString();

/** A colour's shape centred on (cx, cy), `size` across. */
export function shapePath(colour: Colour, cx: number, cy: number, size: number): string {
  const r = size / 2;
  switch (SHAPE_OF[colour]) {
    case 'circle':
      return `M${f(cx - r)} ${f(cy)}a${f(r)} ${f(r)} 0 1 0 ${f(2 * r)} 0a${f(r)} ${f(r)} 0 1 0 ${f(-2 * r)} 0Z`;
    case 'diamond':
      return `M${f(cx)} ${f(cy - r * 1.12)}L${f(cx + r)} ${f(cy)}L${f(cx)} ${f(cy + r * 1.12)}L${f(cx - r)} ${f(cy)}Z`;
    case 'triangle': {
      // Optically centred: a triangle's weight sits low, so it is lifted.
      const top = cy - r * 1.05;
      const base = cy + r * 0.8;
      return `M${f(cx)} ${f(top)}L${f(cx + r * 1.08)} ${f(base)}L${f(cx - r * 1.08)} ${f(base)}Z`;
    }
    case 'square': {
      const s = r * 0.9;
      const k = s * 0.22;
      return (
        `M${f(cx - s + k)} ${f(cy - s)}H${f(cx + s - k)}Q${f(cx + s)} ${f(cy - s)} ${f(cx + s)} ${f(cy - s + k)}` +
        `V${f(cy + s - k)}Q${f(cx + s)} ${f(cy + s)} ${f(cx + s - k)} ${f(cy + s)}H${f(cx - s + k)}` +
        `Q${f(cx - s)} ${f(cy + s)} ${f(cx - s)} ${f(cy + s - k)}V${f(cy - s + k)}Q${f(cx - s)} ${f(cy - s)} ${f(cx - s + k)} ${f(cy - s)}Z`
      );
    }
  }
}

/** Where a numeral sits inside its shape: a triangle's room is low down. */
export function numeralCentre(colour: Colour, cy: number, size: number): number {
  return SHAPE_OF[colour] === 'triangle' ? cy + size * 0.12 : cy;
}

// --- the pictures behind the cards -------------------------------------------

/** A rounded rectangle, for the panel each picture is drawn inside. */
export function roundedRect(x: number, y: number, w: number, h: number, r: number): string {
  return (
    `M${f(x + r)} ${f(y)}H${f(x + w - r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + w)} ${f(y + r)}` +
    `V${f(y + h - r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + w - r)} ${f(y + h)}H${f(x + r)}` +
    `A${f(r)} ${f(r)} 0 0 1 ${f(x)} ${f(y + h - r)}V${f(y + r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + r)} ${f(y)}Z`
  );
}

/** Coral: a sunburst — alternating rays from the middle, and two halos. */
function sunburst(h: number, ink: Inks): Piece[] {
  const cx = 50;
  const cy = h / 2;
  const R = h;
  const rays = 22;
  let d = '';
  for (let i = 0; i < rays; i += 2) {
    const a0 = (i / rays) * 2 * Math.PI;
    const a1 = ((i + 1) / rays) * 2 * Math.PI;
    d +=
      `M${f(cx)} ${f(cy)}L${f(cx + R * Math.cos(a0))} ${f(cy + R * Math.sin(a0))}` +
      `L${f(cx + R * Math.cos(a1))} ${f(cy + R * Math.sin(a1))}Z`;
  }
  return [
    { d, fill: ink.tint, opacity: 0.55 },
    { d: shapePath('C', cx, cy, 64), stroke: ink.tint, sw: 2.5, opacity: 0.9 },
    { d: shapePath('C', cx, cy, 86), stroke: ink.tint, sw: 1.2, opacity: 0.7 },
  ];
}

/** Teal: the sea — rolling bands of waves, darker towards the bottom. */
function sea(h: number, ink: Inks): Piece[] {
  const out: Piece[] = [];
  const step = 13;
  for (let row = 0, y = 6; y < h + step; row++, y += step) {
    const shift = row % 2 ? -6 : 0;
    let d = `M${f(-12 + shift)} ${f(y)}`;
    for (let x = -12 + shift; x < 112; x += 12) {
      d += `q3 -5 6 0t6 0`;
    }
    d += `V${f(h + 4)}H${f(-12 + shift)}Z`;
    // Later rows paint over earlier ones, so each band shows only its crest.
    const depth = y / h;
    out.push({ d, fill: depth > 0.5 ? ink.tint : ink.pale, opacity: 0.35 + 0.4 * depth });
    out.push({ d: d.split('V')[0], stroke: ink.tint, sw: 1.4, opacity: 0.9 });
  }
  return out;
}

/** Violet: a night sky — stars, a crescent moon, and hills along the bottom. */
function night(h: number, ink: Inks): Piece[] {
  const stars: [number, number, number][] = [
    [18, 0.12, 2.4], [38, 0.07, 1.6], [61, 0.15, 2.8], [84, 0.1, 1.8], [12, 0.3, 1.5],
    [30, 0.24, 2.0], [88, 0.3, 2.4], [72, 0.36, 1.4], [22, 0.44, 1.2], [92, 0.5, 1.6],
    [8, 0.58, 2.2], [46, 0.3, 1.2],
  ];
  let sd = '';
  for (const [x, yy, r] of stars) {
    const y = yy * h;
    // A four-pointed twinkle.
    sd +=
      `M${f(x)} ${f(y - r * 2)}Q${f(x)} ${f(y)} ${f(x + r * 2)} ${f(y)}Q${f(x)} ${f(y)} ${f(x)} ${f(y + r * 2)}` +
      `Q${f(x)} ${f(y)} ${f(x - r * 2)} ${f(y)}Q${f(x)} ${f(y)} ${f(x)} ${f(y - r * 2)}Z`;
  }
  const mx = 76;
  const my = h * 0.2;
  const moon = `M${f(mx)} ${f(my - 9)}A9 9 0 1 0 ${f(mx)} ${f(my + 9)}A7 7 0 1 1 ${f(mx)} ${f(my - 9)}Z`;
  const far = `M-2 ${f(h * 0.78)}L18 ${f(h * 0.66)}L34 ${f(h * 0.74)}L56 ${f(h * 0.6)}L78 ${f(h * 0.72)}L102 ${f(h * 0.64)}V${f(h + 2)}H-2Z`;
  const near = `M-2 ${f(h * 0.86)}Q24 ${f(h * 0.76)} 46 ${f(h * 0.84)}T102 ${f(h * 0.8)}V${f(h + 2)}H-2Z`;
  const caps =
    `M56 ${f(h * 0.6)}L61.5 ${f(h * 0.63)}L58 ${f(h * 0.635)}L55 ${f(h * 0.625)}L51 ${f(h * 0.63)}Z` +
    `M18 ${f(h * 0.66)}L22.5 ${f(h * 0.685)}L19 ${f(h * 0.69)}L14 ${f(h * 0.685)}Z`;
  return [
    { d: sd, fill: ink.tint, opacity: 0.95 },
    { d: moon, fill: ink.tint, opacity: 0.95 },
    { d: far, fill: ink.tint, opacity: 0.55 },
    { d: caps, fill: '#FFFFFF', opacity: 0.85 },
    { d: near, fill: ink.tint, opacity: 0.85 },
  ];
}

/** Amber: a honeycomb, with a few cells filled. */
function hive(h: number, ink: Inks): Piece[] {
  const r = 8;
  const w = Math.sqrt(3) * r;
  let outline = '';
  let filled = '';
  let n = 0;
  for (let row = 0, y = 0; y < h + r; row++, y += r * 1.5) {
    for (let x = row % 2 ? w / 2 : 0; x < 100 + w; x += w) {
      let d = '';
      for (let k = 0; k < 6; k++) {
        const a = (Math.PI / 3) * k + Math.PI / 6;
        d += `${k ? 'L' : 'M'}${f(x + r * Math.cos(a))} ${f(y + r * Math.sin(a))}`;
      }
      d += 'Z';
      outline += d;
      // A scatter of honey, fixed rather than random so every copy matches.
      if ((row * 7 + n * 3) % 11 === 0) filled += d;
      n++;
    }
  }
  return [
    { d: filled, fill: ink.tint, opacity: 0.75 },
    { d: outline, stroke: ink.tint, sw: 1.6, opacity: 0.95 },
  ];
}

/** The picture behind a coloured card. */
export function backdrop(colour: Colour, h: number): Piece[] {
  const ink = INKS[colour];
  switch (colour) {
    case 'C':
      return sunburst(h, ink);
    case 'T':
      return sea(h, ink);
    case 'V':
      return night(h, ink);
    case 'A':
      return hive(h, ink);
  }
}

/** The wilds' sky: a scatter of small stars on dusk. */
export function duskStars(h: number): Piece[] {
  const pts: [number, number, number][] = [
    [14, 0.08, 1.1], [33, 0.16, 0.8], [57, 0.06, 1.2], [80, 0.14, 0.9], [92, 0.3, 1.1],
    [8, 0.36, 0.8], [26, 0.84, 1.0], [70, 0.9, 1.2], [88, 0.72, 0.8], [48, 0.94, 0.9],
    [12, 0.66, 1.1], [62, 0.22, 0.7], [40, 0.78, 0.7],
  ];
  let d = '';
  for (const [x, yy, r] of pts) {
    const y = yy * h;
    d += `M${f(x - r)} ${f(y)}a${f(r)} ${f(r)} 0 1 0 ${f(2 * r)} 0a${f(r)} ${f(r)} 0 1 0 ${f(-2 * r)} 0Z`;
  }
  return [{ d, fill: DUSK.star, opacity: 0.7 }];
}

/**
 * The wilds' emblem: a colour wheel — four quarters, one per colour, each
 * carrying its colour's shape in cream, turned a quarter-step so it reads as
 * a wheel rather than a flag. Every colour at once, which is what a wild is.
 */
export function pinwheel(cx: number, cy: number, size: number, named?: Colour): Piece[] {
  const out: Piece[] = [];
  const r = size / 2;
  const turn = -Math.PI / 4;
  COLOURS.forEach((c, i) => {
    const a0 = turn + (Math.PI / 2) * i;
    const a1 = a0 + Math.PI / 2;
    const p0 = { x: cx + r * Math.cos(a0), y: cy + r * Math.sin(a0) };
    const p1 = { x: cx + r * Math.cos(a1), y: cy + r * Math.sin(a1) };
    // Once the wild has named its colour, the other three fade back.
    const faded = named !== undefined && c !== named ? 0.2 : undefined;
    out.push({
      d: `M${f(cx)} ${f(cy)}L${f(p0.x)} ${f(p0.y)}A${f(r)} ${f(r)} 0 0 1 ${f(p1.x)} ${f(p1.y)}Z`,
      fill: INKS[c].main,
      opacity: faded,
    });
    const mid = (a0 + a1) / 2;
    out.push({
      d: shapePath(c, cx + r * 0.55 * Math.cos(mid), cy + r * 0.55 * Math.sin(mid), size * 0.2),
      fill: CREAM,
      opacity: faded,
    });
  });
  // The rim, and the hub the quarters meet at — where a named colour's own
  // shape sits, large, so the card says which colour from across the table.
  out.push({ d: shapePath('C', cx, cy, size), stroke: named ? INKS[named].main : CREAM, sw: Math.max(1, size * (named ? 0.07 : 0.04)) });
  if (named) {
    out.push({ d: shapePath(named, cx, cy, size * 0.46), fill: INKS[named].main, stroke: CREAM, sw: Math.max(1, size * 0.04) });
  } else {
    out.push({ d: shapePath('C', cx, cy, size * 0.16), fill: DUSK.field, stroke: CREAM, sw: Math.max(0.6, size * 0.025) });
  }
  return out;
}

/** Four small shapes in a 2×2 block, for a wild's corner. */
export function shapeBlock(cx: number, cy: number, size: number): Piece[] {
  const s = size / 2;
  const at: [number, number][] = [
    [-1, -1],
    [1, -1],
    [-1, 1],
    [1, 1],
  ];
  return COLOURS.map((c, i) => ({
    d: shapePath(c, cx + (at[i][0] * s) / 2, cy + (at[i][1] * s) / 2, s * 0.86),
    fill: INKS[c].main,
  }));
}

// --- the action glyphs -------------------------------------------------------

/**
 * Skip: an arrow hopping clean over a dot — the player it passes by.
 * Centred on (cx, cy), `size` across.
 */
export function skipGlyph(cx: number, cy: number, size: number, ink: string): Piece[] {
  const u = size / 40;
  const p = (x: number, y: number) => `${f(cx + x * u)} ${f(cy + y * u)}`;
  // The arrowhead sits on the arc's end, along its last tangent — from the
  // control point (0, -24) to the end (12, 2) — so it reads as one stroke.
  return [
    { d: `M${p(-15, 8)}Q${p(0, -24)} ${p(12, 2)}`, stroke: ink, sw: 5.5 * u },
    { d: `M${p(16.5, 11)}L${p(4.6, 4.2)}L${p(17.8, -2.2)}Z`, fill: ink, stroke: ink, sw: 1.5 * u },
    { d: shapePath('C', cx - 1.5 * u, cy + 12 * u, 9 * u), fill: ink },
  ];
}

/** Reverse: one arrow that turns right round on itself. */
export function reverseGlyph(cx: number, cy: number, size: number, ink: string): Piece[] {
  const u = size / 40;
  const p = (x: number, y: number) => `${f(cx + x * u)} ${f(cy + y * u)}`;
  return [
    { d: `M${p(-10, 16)}V${p(-10, -2).split(' ')[1]}A${f(10 * u)} ${f(10 * u)} 0 0 1 ${p(10, -2)}V${f(cy + 4 * u)}`, stroke: ink, sw: 5.5 * u },
    { d: `M${p(1, 2)}L${p(19, 2)}L${p(10, 16)}Z`, fill: ink, stroke: ink, sw: 1.5 * u },
  ];
}
