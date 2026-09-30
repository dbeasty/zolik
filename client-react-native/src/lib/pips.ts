/**
 * Where the pips go on a numbered card.
 *
 * A printed card is not "the suit symbol, N times". The arrangements are
 * fixed, they are older than any of us, and getting them wrong is the single
 * most obvious way a drawn card reads as a drawn card rather than a card: a
 * seven with its pips evenly spaced looks like a spreadsheet, and a seven with
 * the odd pip sitting high between the top pair looks like a seven.
 *
 * Two rules hold across every layout here, and they are the whole of why
 * these read as real:
 *
 *  1. **Two columns and a spine.** Pips sit at a quarter, the middle, or
 *     three quarters across. Nothing sits anywhere else.
 *  2. **The bottom half is upside down.** A real card is printed to be read
 *     from either end of the table, so every pip below the waist is rotated
 *     180°. On a symmetric suit (diamonds) that is invisible; on hearts and
 *     spades it is the difference between a card and a poster of one.
 *
 * Coordinates are fractions of the pip *field* — the area inside the corner
 * indices, not the whole card — so one table serves every card size. The
 * component that draws them (`DeluxeFace`) owns where that field is; this
 * file owns what goes in it, which is what makes it a table of numbers with
 * a test rather than geometry buried in a render.
 */

export type Pip = {
  /** Fraction across the pip field, 0 (left edge) to 1 (right edge). */
  x: number;
  /** Fraction down the pip field. */
  y: number;
  /** Drawn rotated 180°, the way the bottom half of a real card is printed. */
  flip: boolean;
};

/** The two columns, and the spine between them. */
const LEFT = 0.5 - 0.29;
const RIGHT = 0.5 + 0.29;
const MID = 0.5;

/**
 * The four row heights a column of pips uses. `TOP`/`BOTTOM` are where the
 * outer pairs sit on every card from four up; `UPPER`/`LOWER` are the
 * in-between rows a nine and a ten need to fit four down a column.
 *
 * Symmetric about the waist by construction — `BOTTOM` is `1 - TOP` — because
 * a card whose bottom half is not the mirror of its top half is exactly what
 * rule 2 above is there to prevent.
 */
const TOP = 0.09;
const BOTTOM = 1 - TOP;
const UPPER = TOP + (BOTTOM - TOP) / 3;
const LOWER = BOTTOM - (BOTTOM - TOP) / 3;
const WAIST = 0.5;

/** Halfway between the top row and the waist — where a seven's odd pip goes. */
const SHOULDER = (TOP + WAIST) / 2;
const HIP = 1 - SHOULDER;

/** Where a ten's two spine pips sit: between the top row and the upper row. */
const TEN_UPPER = (TOP + UPPER) / 2;
const TEN_LOWER = 1 - TEN_UPPER;

function pip(x: number, y: number): Pip {
  // The waist is the dividing line, and a pip exactly on it is not flipped: a
  // three's middle pip points the same way as its top one on a real card.
  return { x, y, flip: y > WAIST };
}

/** The outer pairs every card from four up shares. */
const CORNERS: Pip[] = [
  pip(LEFT, TOP),
  pip(RIGHT, TOP),
  pip(LEFT, BOTTOM),
  pip(RIGHT, BOTTOM),
];

/** The middle pair a six and up add to the corners. */
const FLANKS: Pip[] = [pip(LEFT, WAIST), pip(RIGHT, WAIST)];

/** Four down each column, for the nine and the ten. */
const COLUMNS: Pip[] = [
  pip(LEFT, TOP),
  pip(RIGHT, TOP),
  pip(LEFT, UPPER),
  pip(RIGHT, UPPER),
  pip(LEFT, LOWER),
  pip(RIGHT, LOWER),
  pip(LEFT, BOTTOM),
  pip(RIGHT, BOTTOM),
];

/**
 * The standard arrangements, by rank.
 *
 * An ace is deliberately absent rather than a one-pip entry: an ace's pip is
 * drawn large and alone at the centre, which is a different piece of art
 * rather than the same table with one row — see `DeluxeFace`. Courts and
 * jokers are absent for the same reason.
 */
const LAYOUTS: Record<string, Pip[]> = {
  '2': [pip(MID, TOP), pip(MID, BOTTOM)],
  '3': [pip(MID, TOP), pip(MID, WAIST), pip(MID, BOTTOM)],
  '4': CORNERS,
  '5': [...CORNERS, pip(MID, WAIST)],
  '6': [...CORNERS, ...FLANKS],
  // The seven's odd pip rides high on the spine, between the top pair — not
  // at the centre, which is where a six-plus-one would put it.
  '7': [...CORNERS, ...FLANKS, pip(MID, SHOULDER)],
  '8': [...CORNERS, ...FLANKS, pip(MID, SHOULDER), pip(MID, HIP)],
  '9': [...COLUMNS, pip(MID, WAIST)],
  '10': [...COLUMNS, pip(MID, TEN_UPPER), pip(MID, TEN_LOWER)],
};

/**
 * The pips for a rank, or an empty list for a rank drawn some other way (an
 * ace, a court, a joker). Callers switch on emptiness rather than re-deriving
 * which ranks are special, so there is one list of that and it is here.
 */
export function pipsFor(rank: string): readonly Pip[] {
  return LAYOUTS[rank] ?? [];
}

/** Ranks that wear a court figure rather than pips. */
export const COURT_RANKS: readonly string[] = ['J', 'Q', 'K'];

export function isCourt(rank: string): boolean {
  return COURT_RANKS.includes(rank);
}
