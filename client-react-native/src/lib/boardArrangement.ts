/**
 * Where the board's regions go — one column, or two side by side on a screen
 * wide enough to hold them.
 *
 * A region is a whole part of the board a player thinks of as one thing: the
 * seats, the table, their hand with the buttons under it, the melds, the
 * opponents' cards, the round results. The cards inside each are still laid
 * out by the module's own zones (see `BoardLayout`); this only says which
 * column each region sits in and in what order. Like the panels a player put
 * away, it is a view preference that lives on the device and never reaches a
 * module.
 *
 * An arrangement starts from a preset and may be nudged from there a region
 * at a time. Every arrangement says where *every* region goes, including ones
 * the game in front does not have — a region with nothing in it draws nothing
 * — so one stored arrangement suits every deal of a game, and a region added
 * to the shell later finds its place by `normalize` rather than vanishing.
 */

export type RegionId = 'results' | 'seats' | 'table' | 'log' | 'hand' | 'spreads' | 'opponents';

export const REGIONS: readonly RegionId[] = ['results', 'seats', 'table', 'log', 'hand', 'spreads', 'opponents'];

export type PresetId = 'stacked' | 'beside' | 'side';

export const PRESETS: readonly PresetId[] = ['stacked', 'beside', 'side'];

export type Arrangement = {
  /** The preset this started from — what the layout picker shows as chosen until a region is moved. */
  preset: PresetId;
  /** Moved by hand since the preset was picked. */
  custom: boolean;
  /** Left column first. A one-column arrangement leaves the second empty. */
  columns: [RegionId[], RegionId[]];
  /** The left column's share of the width, when there are two. */
  split: number;
};

/**
 * Today's single column. The melds go under the buttons where the viewer
 * holds a hand — the hand is what they play from, the melds a record of what
 * is down — and above the hand where nobody holds one (blackjack, where the
 * spreads *are* the cards in play). See `BoardLayout`.
 */
function stackedOrder(holdsHand: boolean): RegionId[] {
  return holdsHand
    ? ['results', 'seats', 'table', 'log', 'hand', 'spreads', 'opponents']
    : ['results', 'seats', 'table', 'log', 'spreads', 'hand', 'opponents'];
}

export function preset(id: PresetId, holdsHand: boolean): Arrangement {
  switch (id) {
    // The table and the seats round it on the left, the player's own cards
    // and everyone's melds on the right: the two things the eye moves
    // between, level with each other instead of one under the other.
    case 'beside':
      return {
        preset: id,
        custom: false,
        columns: [
          ['results', 'seats', 'table', 'log'],
          holdsHand ? ['hand', 'spreads', 'opponents'] : ['spreads', 'hand', 'opponents'],
        ],
        split: 0.5,
      };
    // The board as it is, with the record of the game — results, melds and
    // the opponents' cards — in a narrower column beside it.
    case 'side':
      return {
        preset: id,
        custom: false,
        columns: [
          holdsHand ? ['seats', 'table', 'hand'] : ['seats', 'table', 'spreads', 'hand'],
          holdsHand ? ['results', 'log', 'spreads', 'opponents'] : ['results', 'log', 'opponents'],
        ],
        split: 0.66,
      };
    case 'stacked':
    default:
      return { preset: 'stacked', custom: false, columns: [stackedOrder(holdsHand), []], split: 1 };
  }
}

export const DEFAULT_PRESET: PresetId = 'stacked';

/**
 * The layout a board has until the player picks one. A Mac game window has the
 * width and not the height: stacked, Hold'em needs some 916 points from the
 * piles to the controls, so the table goes beside the hand. Everywhere else,
 * and in a game window too narrow for two columns, stays one column.
 */
export function preferredPreset(gameWindow: boolean, roomForTwo: boolean): PresetId {
  return gameWindow && roomForTwo ? 'beside' : DEFAULT_PRESET;
}

/**
 * A stored arrangement made whole: unknown regions dropped, duplicates kept
 * once, and any region it does not mention put back where its preset has it
 * — so an arrangement saved before a region existed still draws that region.
 * Anything unreadable is the default.
 */
export function normalize(raw: unknown, holdsHand: boolean, fallback: PresetId = DEFAULT_PRESET): Arrangement {
  const r = raw as Partial<Arrangement> | null | undefined;
  const presetId = r && PRESETS.includes(r.preset as PresetId) ? (r.preset as PresetId) : fallback;
  const base = preset(presetId, holdsHand);
  if (!r || !r.custom || !Array.isArray(r.columns) || r.columns.length !== 2) return base;
  const seen = new Set<RegionId>();
  const columns = r.columns.map((col) =>
    (Array.isArray(col) ? col : []).filter((id): id is RegionId => {
      if (!REGIONS.includes(id as RegionId) || seen.has(id as RegionId)) return false;
      seen.add(id as RegionId);
      return true;
    }),
  ) as [RegionId[], RegionId[]];
  for (const id of REGIONS) {
    if (seen.has(id)) continue;
    // Just after whatever its preset has above it, so a region added since —
    // the move log — turns up where it belongs rather than at the foot.
    const c = base.columns[0].includes(id) ? 0 : 1;
    const order = base.columns[c];
    let at = 0;
    for (let k = order.indexOf(id) - 1; k >= 0; k--) {
      const j = columns[c].indexOf(order[k]!);
      if (j >= 0) {
        at = j + 1;
        break;
      }
    }
    columns[c].splice(at, 0, id);
  }
  const split = typeof r.split === 'number' && r.split > 0 && r.split <= 1 ? r.split : base.split;
  return {
    preset: presetId,
    custom: true,
    columns,
    split: columns[1].length && columns[0].length ? Math.min(SPLIT_MAX, Math.max(SPLIT_MIN, split)) : 1,
  };
}

export type Move = 'up' | 'down' | 'left' | 'right';

/** Where a region is: its column, and its place in it. */
export function locate(a: Arrangement, id: RegionId): { column: 0 | 1; index: number } {
  const i = a.columns[0].indexOf(id);
  return i >= 0 ? { column: 0, index: i } : { column: 1, index: a.columns[1].indexOf(id) };
}

/**
 * Whether a move goes anywhere, among the regions this game draws. A move
 * past a region the game does not have would look like nothing happened, so
 * up and down step over those, and a move that has nowhere to go is refused
 * rather than taken.
 */
export function canMove(a: Arrangement, id: RegionId, move: Move, present: ReadonlySet<RegionId>): boolean {
  const { column, index } = locate(a, id);
  if (index < 0) return false;
  if (move === 'left') return column === 1;
  if (move === 'right') return column === 0;
  const col = a.columns[column];
  const others = move === 'up' ? col.slice(0, index) : col.slice(index + 1);
  return others.some((r) => present.has(r));
}

/**
 * One step for one region. Up and down swap it past the next region the game
 * actually draws; left and right move it to the other column, at the same
 * height as near as there is one. Moving a region out of a column leaves the
 * column where it stands, however short; a second column appears on the
 * first move right, at half the width.
 */
export function move(a: Arrangement, id: RegionId, m: Move, present: ReadonlySet<RegionId>): Arrangement {
  if (!canMove(a, id, m, present)) return a;
  const { column, index } = locate(a, id);
  const columns: [RegionId[], RegionId[]] = [[...a.columns[0]], [...a.columns[1]]];
  const col = columns[column];
  if (m === 'up' || m === 'down') {
    const step = m === 'up' ? -1 : 1;
    let j = index + step;
    while (!present.has(col[j]!)) j += step;
    col.splice(index, 1);
    col.splice(j, 0, id);
  } else {
    const to = m === 'left' ? 0 : 1;
    col.splice(index, 1);
    columns[to].splice(Math.min(index, columns[to].length), 0, id);
  }
  let split = a.split;
  if (!columns[1].length) split = 1;
  else if (!a.columns[1].length) split = 0.5;
  return { ...a, custom: true, columns, split };
}

/**
 * One region put somewhere in particular — where a dragged panel was let go:
 * into `column`, just above `before`, or at the foot of the column when
 * `before` is null. Letting go where it already was changes nothing. A second
 * column appears at half the width, and emptying one closes it, as with a
 * step to the side.
 */
export function moveTo(a: Arrangement, id: RegionId, column: 0 | 1, before: RegionId | null): Arrangement {
  if (before === id) return a;
  const from = locate(a, id);
  if (from.index < 0) return a;
  const columns: [RegionId[], RegionId[]] = [[...a.columns[0]], [...a.columns[1]]];
  columns[from.column].splice(from.index, 1);
  const to = columns[column];
  const at = before ? to.indexOf(before) : -1;
  to.splice(at >= 0 ? at : to.length, 0, id);
  if (columns[0].join() === a.columns[0].join() && columns[1].join() === a.columns[1].join()) return a;
  let split = a.split;
  if (!columns[1].length || !columns[0].length) split = 1;
  else if (!a.columns[1].length || !a.columns[0].length) split = 0.5;
  return { ...a, custom: true, columns, split };
}

/**
 * The arrangement a player is moving regions about in: the one stored, or —
 * where two columns would not fit and the board is drawn as one — that one
 * column, in the order it is drawn. A step up or down taken in the stored
 * columns would move a region somewhere this screen does not show, which is
 * a press that changes nothing the player can see.
 */
export function asDrawn(a: Arrangement, roomForTwo: boolean, holdsHand: boolean): Arrangement {
  if (roomForTwo || !a.columns[0].length || !a.columns[1].length) return a;
  return { ...a, custom: true, columns: [stackedOrder(holdsHand), []], split: 1 };
}

/** The narrowest and widest the left column may be made, as a share of the board. */
export const SPLIT_MIN = 0.25;
export const SPLIT_MAX = 0.75;

/**
 * Halves and thirds are what people aim for, so a divider let go near one
 * lands on it: within this much of the board's width either side.
 */
const SNAP = 0.025;
const SNAPS = [1 / 3, 0.5, 2 / 3];

/** A dragged divider's share, kept within reach of both columns and caught at halves and thirds. */
export function clampSplit(split: number): number {
  const s = Math.min(SPLIT_MAX, Math.max(SPLIT_MIN, split));
  const near = SNAPS.find((p) => Math.abs(p - s) <= SNAP);
  return near ?? s;
}

/** The arrangement with the left column at `split` of the width. Changes nothing with one column. */
export function withSplit(a: Arrangement, split: number): Arrangement {
  if (!a.columns[1].length || !a.columns[0].length) return a;
  const s = clampSplit(split);
  return s === a.split ? a : { ...a, custom: true, split: s };
}

/**
 * What a screen actually draws. Two columns only when the board has the room
 * and both have something in them; otherwise one, in the order the player
 * set when the arrangement is a single column, and today's order when it is
 * two — a side-by-side layout read top to bottom would put the results under
 * the hand, which is no order anyone chose.
 *
 * While the player is arranging the board, both columns are drawn wherever
 * there is room, an empty one too: it is somewhere to drop a panel, and a
 * column that was not drawn could not be dropped into.
 */
export function columnsToDraw(
  a: Arrangement,
  roomForTwo: boolean,
  present: ReadonlySet<RegionId>,
  holdsHand: boolean,
  editing = false,
): { columns: RegionId[][]; split: number } {
  const left = a.columns[0].filter((r) => present.has(r));
  const right = a.columns[1].filter((r) => present.has(r));
  if (editing && roomForTwo) {
    return { columns: [left, right], split: left.length && right.length ? a.split : left.length ? 0.7 : 0.3 };
  }
  if (!right.length) return { columns: [left], split: 1 };
  if (!left.length) return { columns: [right], split: 1 };
  if (roomForTwo) return { columns: [left, right], split: a.split };
  return { columns: [stackedOrder(holdsHand).filter((r) => present.has(r))], split: 1 };
}

/**
 * The narrowest board, in points, two columns are drawn on. Under it a hand
 * of thirteen at the scale the window gives would be squeezed into an
 * overlap the player did not ask for. Lower than it was (1100): a laptop
 * window is 900 to 1100 wide, and there two columns were offered and never
 * drawn — every move to the side looked like nothing happening. The hand
 * measures the column it is given, so it fans tighter rather than spilling.
 */
export const TWO_COLUMN_MIN_WIDTH = 900;

/**
 * The widest a two-column board is drawn. A single column stops at a table's
 * width (`BOARD_MAX_WIDTH`), because a hand stretched past it is just gaps;
 * two columns are two tables' worth of things, so they may take more of a
 * large monitor before the rest goes to the felt.
 */
export const TWO_COLUMN_MAX_WIDTH = 2200;
