/**
 * The Heirloom Last Card pack, as drawing data: the same four colours and
 * shapes as the casino pack (`lastCardArt.ts`), engraved — ivory stock, a
 * double rule with corner flourishes, fine hatching, and pictures drawn in
 * line as much as in fill: a sun with straight and wavy rays over clouds,
 * curling waves with gulls, a cratered moon over hatched hills and pines, a
 * honeycomb with honey and bees. The back is navy and gold filigree.
 *
 * Geometry only, in the same 100-wide field the other faces use.
 */

import { COLOURS, INKS, shapePath, type Colour, type Inks, type Piece } from '@/src/components/cards/lastCardArt';

/** Heirloom's own inks: ivory stock, a gilt rule, the navy of its tables. */
export const HEIRLOOM = {
  stock: '#FBF4E2',
  gilt: '#B8913A',
  giltLight: '#E3C77A',
  navy: '#14213D',
  navyLight: '#24365E',
  ink: '#2B2433',
};

const f = (n: number) => (Math.round(n * 100) / 100).toString();

function circle(cx: number, cy: number, r: number): string {
  return `M${f(cx - r)} ${f(cy)}a${f(r)} ${f(r)} 0 1 0 ${f(2 * r)} 0a${f(r)} ${f(r)} 0 1 0 ${f(-2 * r)} 0Z`;
}

// --- the frame -------------------------------------------------------------------

/** A double rule round the card, with a scroll in each corner. */
export function ornateFrame(h: number, ink: string): Piece[] {
  const outer = rect(3, 3, 94, h - 6, 6);
  const inner = rect(6.5, 6.5, 87, h - 13, 4);
  let curls = '';
  for (const [x, y, sx, sy] of [
    [6.5, 6.5, 1, 1],
    [93.5, 6.5, -1, 1],
    [6.5, h - 6.5, 1, -1],
    [93.5, h - 6.5, -1, -1],
  ] as const) {
    // A small scroll: out along the rule, curled back in.
    curls +=
      `M${f(x + sx * 2)} ${f(y + sy * 12)}C${f(x + sx * 2)} ${f(y + sy * 4)} ${f(x + sx * 4)} ${f(y + sy * 2)} ${f(x + sx * 12)} ${f(y + sy * 2)}` +
      `M${f(x + sx * 5)} ${f(y + sy * 9)}c${f(sx * 0)} ${f(sy * -2.5)} ${f(sx * 2)} ${f(sy * -4)} ${f(sx * 4)} ${f(sy * -4)}`;
  }
  return [
    { d: outer, stroke: HEIRLOOM.gilt, sw: 1.6 },
    { d: inner, stroke: ink, sw: 0.9 },
    { d: curls, stroke: HEIRLOOM.gilt, sw: 1.1 },
  ];
}

function rect(x: number, y: number, w: number, h: number, r: number): string {
  return (
    `M${f(x + r)} ${f(y)}H${f(x + w - r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + w)} ${f(y + r)}` +
    `V${f(y + h - r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + w - r)} ${f(y + h)}H${f(x + r)}` +
    `A${f(r)} ${f(r)} 0 0 1 ${f(x)} ${f(y + h - r)}V${f(y + r)}A${f(r)} ${f(r)} 0 0 1 ${f(x + r)} ${f(y)}Z`
  );
}

/** Fine diagonal hatching, the engraver's shading, across the whole field. */
export function hatching(h: number, ink: string, step = 3.2): Piece {
  let d = '';
  for (let x = -h; x < 100; x += step) d += `M${f(x)} ${f(h)}L${f(x + h)} 0`;
  return { d, stroke: ink, sw: 0.35, opacity: 0.35 };
}

// --- the four pictures --------------------------------------------------------------

/** Coral: a sun of straight and wavy rays, a dotted halo, clouds below. */
function sun(h: number, ink: Inks): Piece[] {
  const cx = 50;
  const cy = h * 0.42;
  let straight = '';
  let wavy = '';
  const n = 24;
  for (let i = 0; i < n; i++) {
    const a = (i / n) * 2 * Math.PI;
    const r0 = 21;
    const r1 = i % 2 ? 44 : 52;
    const x0 = cx + r0 * Math.cos(a);
    const y0 = cy + r0 * Math.sin(a);
    const x1 = cx + r1 * Math.cos(a);
    const y1 = cy + r1 * Math.sin(a);
    if (i % 2) {
      straight += `M${f(x0)} ${f(y0)}L${f(x1)} ${f(y1)}`;
    } else {
      // A wavy ray: three bends along its length.
      const nx = -Math.sin(a) * 2.2;
      const ny = Math.cos(a) * 2.2;
      wavy += `M${f(x0)} ${f(y0)}`;
      for (let k = 1; k <= 3; k++) {
        const t = k / 3;
        const s = k % 2 ? 1 : -1;
        const mx = x0 + (x1 - x0) * (t - 1 / 6);
        const my = y0 + (y1 - y0) * (t - 1 / 6);
        wavy += `Q${f(mx + nx * s)} ${f(my + ny * s)} ${f(x0 + (x1 - x0) * t)} ${f(y0 + (y1 - y0) * t)}`;
      }
    }
  }
  let dots = '';
  for (let i = 0; i < 36; i++) {
    const a = (i / 36) * 2 * Math.PI;
    dots += circle(cx + 26 * Math.cos(a), cy + 26 * Math.sin(a), 0.7);
  }
  const clouds =
    `M-2 ${f(h * 0.86)}Q8 ${f(h * 0.76)} 20 ${f(h * 0.82)}Q28 ${f(h * 0.72)} 40 ${f(h * 0.8)}` +
    `Q52 ${f(h * 0.74)} 60 ${f(h * 0.82)}Q72 ${f(h * 0.73)} 84 ${f(h * 0.8)}Q94 ${f(h * 0.76)} 102 ${f(h * 0.84)}V${f(h + 2)}H-2Z`;
  return [
    { d: straight, stroke: ink.tint, sw: 1.4 },
    { d: wavy, stroke: ink.tint, sw: 1.1 },
    { d: dots, fill: ink.main, opacity: 0.7 },
    { d: circle(cx, cy, 21), stroke: ink.tint, sw: 1.2 },
    { d: clouds, fill: HEIRLOOM.stock, stroke: ink.tint, sw: 1 },
  ];
}

/** Teal: rows of waves whose crests curl over, and gulls in the sky. */
function sea(h: number, ink: Inks): Piece[] {
  const out: Piece[] = [];
  const rows = [0.5, 0.62, 0.74, 0.86];
  rows.forEach((yy, row) => {
    const y = h * yy;
    const shift = row % 2 ? 7 : 0;
    let body = `M-4 ${f(y + 6)}`;
    let curls = '';
    for (let x = -14 + shift; x < 110; x += 16) {
      // A crest: up, over, and curled back on itself.
      body += `L${f(x)} ${f(y + 3)}Q${f(x + 6)} ${f(y - 6)} ${f(x + 12)} ${f(y - 1)}`;
      curls += `M${f(x + 12)} ${f(y - 1)}q-2.5 -2.5 -4.5 0q1.5 2 3 0.6`;
    }
    body += `L106 ${f(y + 6)}V${f(h + 2)}H-4Z`;
    out.push({ d: body, fill: row % 2 ? ink.pale : ink.tint, opacity: 0.55 + row * 0.1, stroke: ink.main, sw: 0.9 });
    out.push({ d: curls, stroke: ink.deep, sw: 0.8 });
  });
  let gulls = '';
  for (const [x, y, s] of [
    [24, 0.16, 4],
    [36, 0.22, 3],
    [70, 0.14, 4.5],
    [80, 0.24, 3],
  ] as const) {
    gulls += `M${f(x - s)} ${f(h * y)}q${f(s / 2)} ${f(-s / 2)} ${f(s)} 0q${f(s / 2)} ${f(-s / 2)} ${f(s)} 0`;
  }
  out.push({ d: gulls, stroke: ink.deep, sw: 0.9 });
  return out;
}

/** Violet: a cratered crescent, a constellation, hatched hills and pines. */
function night(h: number, ink: Inks): Piece[] {
  const mx = 74;
  const my = h * 0.17;
  const moon = `M${f(mx)} ${f(my - 10)}A10 10 0 1 0 ${f(mx)} ${f(my + 10)}A8 8 0 1 1 ${f(mx)} ${f(my - 10)}Z`;
  const craters = circle(mx - 5, my - 2, 1.3) + circle(mx - 3, my + 4, 0.9) + circle(mx - 7, my + 3, 0.7);
  const stars: [number, number][] = [
    [16, 0.12],
    [26, 0.2],
    [38, 0.14],
    [48, 0.24],
    [30, 0.3],
  ];
  let lines = '';
  stars.forEach(([x, y], i) => {
    if (i > 0) lines += `M${f(stars[i - 1][0])} ${f(stars[i - 1][1] * h)}L${f(x)} ${f(y * h)}`;
  });
  let dots = '';
  for (const [x, y] of stars) dots += circle(x, y * h, 1.2);
  for (const [x, y] of [
    [60, 0.34],
    [88, 0.36],
    [10, 0.4],
    [52, 0.08],
  ] as const)
    dots += circle(x, y * h, 0.7);
  const far = `M-2 ${f(h * 0.74)}L20 ${f(h * 0.6)}L36 ${f(h * 0.7)}L58 ${f(h * 0.56)}L80 ${f(h * 0.68)}L102 ${f(h * 0.6)}V${f(h + 2)}H-2Z`;
  let hatch = '';
  for (let x = -2; x < 102; x += 2.4) hatch += `M${f(x)} ${f(h * 0.62)}L${f(x + 6)} ${f(h * 0.8)}`;
  let pines = '';
  for (const [x, s] of [
    [14, 1],
    [22, 0.8],
    [80, 1.1],
    [89, 0.85],
  ] as const) {
    const base = h * 0.92;
    pines += `M${f(x)} ${f(base - 18 * s)}L${f(x + 6 * s)} ${f(base - 8 * s)}H${f(x + 3 * s)}L${f(x + 8 * s)} ${f(base)}H${f(x - 8 * s)}L${f(x - 3 * s)} ${f(base - 8 * s)}H${f(x - 6 * s)}Z`;
  }
  const near = `M-2 ${f(h * 0.9)}Q30 ${f(h * 0.82)} 52 ${f(h * 0.9)}T102 ${f(h * 0.86)}V${f(h + 2)}H-2Z`;
  return [
    { d: lines, stroke: ink.main, sw: 0.5, opacity: 0.8 },
    { d: dots, fill: ink.deep },
    { d: moon, fill: HEIRLOOM.stock, stroke: ink.deep, sw: 1 },
    { d: craters, stroke: ink.main, sw: 0.5 },
    { d: far, fill: ink.tint, stroke: ink.main, sw: 0.8, opacity: 0.9 },
    { d: hatch, stroke: ink.main, sw: 0.4, opacity: 0.6 },
    { d: near, fill: ink.main, opacity: 0.85 },
    { d: pines, fill: ink.deep },
  ];
}

/** Amber: a double-ruled honeycomb, honey running from its top, two bees. */
function hive(h: number, ink: Inks): Piece[] {
  const r = 9;
  const w = Math.sqrt(3) * r;
  let cells = '';
  let filled = '';
  let n = 0;
  for (let row = 0, y = 0; y < h + r; row++, y += r * 1.5) {
    for (let x = row % 2 ? w / 2 : 0; x < 100 + w; x += w) {
      for (const rr of [r, r - 1.8]) {
        for (let k = 0; k < 6; k++) {
          const a = (Math.PI / 3) * k + Math.PI / 6;
          cells += `${k ? 'L' : 'M'}${f(x + rr * Math.cos(a))} ${f(y + rr * Math.sin(a))}`;
        }
        cells += 'Z';
      }
      if ((row * 5 + n * 3) % 7 === 0) {
        for (let k = 0; k < 6; k++) {
          const a = (Math.PI / 3) * k + Math.PI / 6;
          filled += `${k ? 'L' : 'M'}${f(x + (r - 1.8) * Math.cos(a))} ${f(y + (r - 1.8) * Math.sin(a))}`;
        }
        filled += 'Z';
      }
      n++;
    }
  }
  let drips = '';
  for (const [x, len] of [
    [14, 14],
    [30, 22],
    [62, 10],
    [84, 18],
  ] as const) {
    drips += `M${f(x - 3)} 0V${f(len)}a3 3 0 0 0 6 0V0Z`;
  }
  return [
    { d: cells, stroke: ink.main, sw: 0.7, opacity: 0.8 },
    { d: filled, fill: ink.tint, opacity: 0.8 },
    { d: drips, fill: ink.main, opacity: 0.85 },
    ...bee(20, h * 0.78, 1),
    ...bee(80, h * 0.24, 0.85),
  ];
}

function bee(x: number, y: number, s: number): Piece[] {
  const body = `M${f(x - 5 * s)} ${f(y)}a${f(5 * s)} ${f(3.4 * s)} 0 1 0 ${f(10 * s)} 0a${f(5 * s)} ${f(3.4 * s)} 0 1 0 ${f(-10 * s)} 0Z`;
  const stripes = `M${f(x - 1.6 * s)} ${f(y - 3.2 * s)}V${f(y + 3.2 * s)}M${f(x + 1.6 * s)} ${f(y - 3.2 * s)}V${f(y + 3.2 * s)}`;
  const wings =
    `M${f(x - 1 * s)} ${f(y - 2.6 * s)}a${f(2.6 * s)} ${f(3.6 * s)} -20 1 1 ${f(-1 * s)} ${f(-0.4 * s)}Z` +
    `M${f(x + 1 * s)} ${f(y - 2.6 * s)}a${f(2.6 * s)} ${f(3.6 * s)} 20 1 0 ${f(1 * s)} ${f(-0.4 * s)}Z`;
  const feelers = `M${f(x + 4.5 * s)} ${f(y - 1.5 * s)}q${f(2 * s)} ${f(-3 * s)} ${f(3.5 * s)} ${f(-3.2 * s)}`;
  return [
    { d: wings, fill: HEIRLOOM.stock, stroke: HEIRLOOM.ink, sw: 0.5, opacity: 0.95 },
    { d: body, fill: '#E5A019', stroke: HEIRLOOM.ink, sw: 0.6 },
    { d: stripes, stroke: HEIRLOOM.ink, sw: 1.1 },
    { d: feelers, stroke: HEIRLOOM.ink, sw: 0.5 },
  ];
}

/** The engraved picture behind a coloured card. */
export function heirloomBackdrop(colour: Colour, h: number): Piece[] {
  const ink = INKS[colour];
  switch (colour) {
    case 'C':
      return sun(h, ink);
    case 'T':
      return sea(h, ink);
    case 'V':
      return night(h, ink);
    case 'A':
      return hive(h, ink);
  }
}

/** A medallion's engraved ring: a dotted rule round the shape. */
export function medallionRing(colour: Colour, cx: number, cy: number, size: number): Piece[] {
  return [
    { d: shapePath(colour, cx, cy, size + 12), stroke: HEIRLOOM.gilt, sw: 0.9 },
    { d: shapePath(colour, cx, cy, size + 7), stroke: INKS[colour].deep, sw: 0.6, opacity: 0.7 },
  ];
}

// --- the back ------------------------------------------------------------------------

/** Navy and gold: a filigree border, a rosette of the four shapes, a ribbon. */
export function heirloomBack(h: number): { pieces: Piece[]; ribbonY: number } {
  const cx = 50;
  const cy = h / 2;
  const out: Piece[] = [
    { d: rect(0, 0, 100, h, 8), fill: HEIRLOOM.navy },
    { d: rect(4, 4, 92, h - 8, 5), stroke: HEIRLOOM.gilt, sw: 1.4 },
    { d: rect(7, 7, 86, h - 14, 3), stroke: HEIRLOOM.giltLight, sw: 0.6, opacity: 0.8 },
  ];
  // A running border of small diamonds between the two rules.
  let border = '';
  for (let x = 12; x <= 88; x += 6) border += diamond(x, 5.5, 1.1) + diamond(x, h - 5.5, 1.1);
  for (let y = 12; y <= h - 12; y += 6) border += diamond(5.5, y, 1.1) + diamond(94.5, y, 1.1);
  out.push({ d: border, fill: HEIRLOOM.giltLight, opacity: 0.9 });
  // A lattice behind everything, very faint.
  let lattice = '';
  for (let x = -h; x < 100; x += 7) lattice += `M${f(x)} ${f(h)}L${f(x + h)} 0M${f(x)} 0L${f(x + h)} ${f(h)}`;
  out.push({ d: lattice, stroke: HEIRLOOM.navyLight, sw: 0.5 });
  // The rosette: eight gilt petals, a ring, and the four shapes in colour.
  let petals = '';
  for (let i = 0; i < 8; i++) {
    const a = (i / 8) * 2 * Math.PI;
    const tx = cx + 24 * Math.cos(a);
    const ty = cy + 24 * Math.sin(a);
    const l = a + 0.35;
    const rr = a - 0.35;
    petals += `M${f(cx)} ${f(cy)}Q${f(cx + 16 * Math.cos(l))} ${f(cy + 16 * Math.sin(l))} ${f(tx)} ${f(ty)}Q${f(cx + 16 * Math.cos(rr))} ${f(cy + 16 * Math.sin(rr))} ${f(cx)} ${f(cy)}Z`;
  }
  out.push({ d: petals, fill: HEIRLOOM.navyLight, stroke: HEIRLOOM.gilt, sw: 0.8 });
  out.push({ d: circle(cx, cy, 13), fill: HEIRLOOM.navy, stroke: HEIRLOOM.gilt, sw: 1.2 });
  COLOURS.forEach((c, i) => {
    const a = (i / 4) * 2 * Math.PI - Math.PI / 2;
    out.push({ d: shapePath(c, cx + 6.5 * Math.cos(a), cy + 6.5 * Math.sin(a), 6), fill: INKS[c].main, stroke: HEIRLOOM.giltLight, sw: 0.4 });
  });
  // Corner fans.
  let fans = '';
  for (const [x, y, sx, sy] of [
    [10, 10, 1, 1],
    [90, 10, -1, 1],
    [10, h - 10, 1, -1],
    [90, h - 10, -1, -1],
  ] as const) {
    for (let k = 0; k < 4; k++) {
      const a0 = (k / 4) * (Math.PI / 2);
      fans += `M${f(x)} ${f(y)}L${f(x + sx * 10 * Math.cos(a0))} ${f(y + sy * 10 * Math.sin(a0))}`;
    }
    fans += `M${f(x + sx * 10)} ${f(y)}A10 10 0 0 ${sx * sy > 0 ? 1 : 0} ${f(x)} ${f(y + sy * 10)}`;
  }
  out.push({ d: fans, stroke: HEIRLOOM.gilt, sw: 0.8 });
  // The ribbon the wordmark sits on.
  const ry = cy + 30;
  out.push({
    d: `M22 ${f(ry - 5)}H78L74 ${f(ry)}L78 ${f(ry + 5)}H22L26 ${f(ry)}Z`,
    fill: '#7A1F2B',
    stroke: HEIRLOOM.gilt,
    sw: 0.8,
  });
  return { pieces: out, ribbonY: ry };
}

function diamond(x: number, y: number, r: number): string {
  return `M${f(x)} ${f(y - r)}L${f(x + r)} ${f(y)}L${f(x)} ${f(y + r)}L${f(x - r)} ${f(y)}Z`;
}
