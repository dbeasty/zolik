/**
 * How big a card, a panel, or a control is, as a function of how wide the
 * screen is.
 *
 * One place decides this so a scaled card and its drag-and-drop drop gap
 * never disagree — see `HandZone`'s `DropGap`, which is built from the same
 * numbers a card is for exactly that reason. Everything below `narrow` is a
 * one-column phone layout.
 *
 * Scale 1 is the phone-and-laptop baseline, not a ceiling. It used to be a
 * ceiling — a 2560px monitor drew exactly the same 52×72 card a 768px laptop
 * did, which left a desktop table looking like a phone screenshot stretched
 * across a wall, with a hand of postage stamps in the middle of it. Above
 * 1024 the cards grow instead (see `scaleFor`), which is the entire point of
 * having the room; `maxWidth` then stops the *board* from growing with them,
 * because a column of panels 2500 pixels wide is not a table, it is a spread
 * sheet.
 */

export type CardMetrics = {
  width: number;
  height: number;
  /** Space after each card, part of what a slot occupies. */
  gap: number;
  /** The ring drawn around every card. A hairline: never scaled. */
  ringPadding: number;
  ringBorder: number;
  compactWidth: number;
  compactHeight: number;
  rankFont: number;
  suitFont: number;
  jokerRankFont: number;
  suitInlineFont: number;
  /**
   * The flex gap between two cards in a fanned hand. A spacing, not a
   * hairline, but it stays 4 at every scale: it reads as "these are separate
   * cards" and that job does not get harder on a bigger screen.
   */
  fanGap: number;
  /**
   * Everything one card of a fanned hand occupies across the row: the card,
   * its own trailing gap, its selection ring, the slot border `HandZone`
   * wraps it in, and the gap to the next slot.
   *
   * Derived here rather than by whoever needs it, because the last two terms
   * are easy to forget and the answer looks plausible without them — which is
   * how the scale ceiling below first got set about 7% too high, and a
   * thirteen-card hand wrapped on the very screens that were supposed to have
   * room for it.
   */
  slotPitch: number;
};

export type PanelMetrics = {
  padding: number;
  gap: number;
  radius: number;
  titleFont: number;
  bodyFont: number;
};

/** How big a player's face is drawn, open and on the collapsed rail. */
export type SeatMetrics = {
  avatar: number;
  avatarCompact: number;
};

export type Metrics = {
  /** 1 on a comfortable screen, less on a small one, more on a big one. */
  scale: number;
  /** A one-column screen: panels stack instead of sitting side by side. */
  narrow: boolean;
  /**
   * Room enough to draw cards at something like their printed size — the
   * screens `scaleFor` takes above 1. Exposed so a component can spend the
   * room on more than size (the dealer takes their own place at the table
   * here, rather than sitting in the row of melds), never so it can decide a
   * different size: that is still `card` below, and still one answer.
   */
  roomy: boolean;
  /**
   * How wide the board itself is allowed to get, however wide the window is.
   * A full-width column on a large monitor puts a player's hand and the draw
   * pile at opposite ends of a metre of glass.
   */
  maxWidth: number;
  card: CardMetrics;
  panel: PanelMetrics;
  seat: SeatMetrics;
  /** Visible corner height of an overlapped card in a stacked group. */
  stackedCorner: number;
  /** Smallest a control may be before it wraps to the next line. */
  buttonMinWidth: number;
};

/** Today's fixed card size, kept as the scale-1 baseline. */
const BASE_CARD = {
  width: 52,
  height: 72,
  gap: 6,
  fanGap: 4,
  ringPadding: 1,
  ringBorder: 2,
  compactWidth: 44,
  compactHeight: 60,
  rankFont: 14,
  suitFont: 18,
  jokerRankFont: 11,
  suitInlineFont: 14,
};

/**
 * How big a player's face is drawn. A size, so it lives here and not in the
 * skin — a skin may repaint a face but never resize one.
 *
 * 40 rather than the 26 the initial-circle used: at 26 a portrait is a dot,
 * and the whole point of a face is that it is recognised across the table
 * without being read. The collapsed rail keeps the smaller one, where the
 * point is only to tell four players apart in a row of pills.
 */
const BASE_SEAT = {
  avatar: 40,
  avatarCompact: 22,
};

const BASE_PANEL = {
  padding: 8,
  gap: 8,
  radius: 10,
  titleFont: 12,
  bodyFont: 12,
};

/**
 * The biggest a card is ever drawn, as a multiple of the 52×72 baseline —
 * 130×180, about the size of a real card held at arm's length.
 *
 * A ceiling rather than a derivation, and the honest reason is that the rule
 * the other sizes come from has run out: once a hand stacks, thirteen cards
 * fit across a 1400px board at a scale near five, and nothing about the
 * *width* of the board stops a card getting there. What stops it is height —
 * every card on the board grows together, the stock and the discard pile
 * included, and the board is a column of panels that has to end above the
 * bottom of the window. There is no window height in these metrics to derive
 * that from, so it is a number, checked by eye at 1440×900 and 2560×1440.
 */
const FULL_CARD = 2.5;

const BASE_BUTTON_MIN_WIDTH = 92;
const BASE_STACKED_CORNER = 26;

/**
 * Breakpoints chosen against a real 375×812 phone viewport, where a 13-card
 * hand at scale 1 cost 407 of a 716px-tall screen. Below 768 the layout goes
 * to one column; below that, cards themselves start shrinking.
 *
 * The rule the growth steps are set by has always been the same one — a
 * thirteen-card hand, a full Žolíky deal and the longest this client draws,
 * has to fit across the board in a single row, because a hand that wraps is
 * a hand you have to re-find every turn. What changed is what "fit" costs.
 *
 * A hand is fanned now: past the point where the cards would wrap they slide
 * over each other instead, the way they do between your fingers, and a card
 * in the middle of the fan costs the width of its own index rather than the
 * width of a card. So the same rule buys a great deal more card. Thirteen
 * stacked cards would fit across a 1400px board at nearly five times the
 * original size — which is the point at which the rule stops being what
 * limits anything, and `FULL_CARD` takes over.
 *
 * So above 1280 the steps climb toward `FULL_CARD` rather than toward
 * whatever crammed thirteen cards in side by side. Below it they are
 * unchanged and deliberately so: a phone's constraint was never the width of
 * the row, it was the 812 pixels of height the whole board has to live in,
 * and making a phone's cards half again as tall to use up spare width would
 * push the controls off the bottom of the screen. Nothing about fanning a
 * hand gives a phone more height.
 */
/** How much of a card still shows once the next one covers it. */
const PEEK_OF_CARD = 0.38;

/**
 * The parts of a slot that do not grow with the card: the selection ring
 * `CardView` draws, and the border of the slot `HandZone` wraps it in. Named
 * here because `slotPitch` is built from the same pieces and the two must not
 * drift — the first time that arithmetic was done by eye the top of the scale
 * came out a step too high and the thirteenth card wrapped.
 */
const SLOT_FIXED =
  2 * (BASE_CARD.ringPadding + BASE_CARD.ringBorder) + 2 * BASE_CARD.ringBorder;

/** A full Žolíky deal: the longest hand this client draws. */
const FULL_HAND = 13;

/**
 * How much room the hand's row has at a given window width.
 *
 * The panel's own padding grows with the chrome scale, which is derived from
 * the card scale, which is what this is used to work out — so the largest
 * padding of the range is taken and the circle broken. It varies by five
 * pixels end to end, which is not worth a fixed point iteration.
 */
function rowRoomAt(width: number): number {
  const board = Math.min(width, BOARD_MAX_WIDTH);
  return board - 2 * SCREEN_PADDING - 2 * Math.round(BASE_PANEL.padding * 1.33);
}

/**
 * How big a card is, given how much room there is for one.
 *
 * Solved rather than stepped. The rule has not changed since the first card
 * was drawn — thirteen of them fit across the board in one row — but for a
 * long time it was applied to a hand laid out side by side, so the card was
 * whatever size let thirteen of them sit in a row without touching. That is
 * backwards: given the choice, a bigger card overlapping its neighbour beats
 * a smaller one that does not, because what you are looking at is a card and
 * what you lose to the overlap is the part of it you were not reading.
 *
 * So the size comes from the closed hand. Thirteen closed cards cost twelve
 * pitches and one whole slot, and every part of both is either a multiple of
 * the scale or a constant, so the largest scale the row can take falls out of
 * one division rather than out of a table of guesses.
 *
 * Two things cap it:
 *
 *  - `FULL_CARD`, because past a point a bigger card is just a bigger card.
 *    The board stops widening at 1400 anyway, so every monitor past that has
 *    the same row and would otherwise get the same answer forever.
 *  - Below 768, the old table. A phone's constraint was never the width of
 *    its row — the breakpoints under there were set against a real 375×812
 *    screen, where a hand cost 407 of 716 usable pixels — and a fanned hand
 *    gives nobody more height. Working the width out and ignoring the height
 *    would hand a phone a card half again as tall and push the controls off
 *    the bottom of the screen.
 */
function scaleFor(width: number): number {
  if (width >= 768) {
    const perScale = (FULL_HAND - 1) * BASE_CARD.width * PEEK_OF_CARD + BASE_CARD.width + BASE_CARD.gap;
    const largest = (rowRoomAt(width) - SLOT_FIXED) / perScale;
    // Rounded down to a tenth: a card is a size, not a measurement, and two
    // windows a pixel apart should not draw cards a pixel apart.
    return Math.min(FULL_CARD, Math.floor(largest * 10) / 10);
  }
  if (width >= 480) return 0.88;
  if (width >= 380) return 0.78;
  return 0.7;
}



/**
 * The widest the board is drawn, whatever the window does past it. 1400 is
 * where a 1.6-scale hand, the piles beside it and the panel padding all still
 * have room; past that the extra pixels go to the felt on either side, which
 * is what a table edge looks like.
 */
const BOARD_MAX_WIDTH = 1400;

/**
 * How much of the card scale the *chrome* takes — panel titles, body text,
 * padding, the minimum width of a button.
 *
 * Cards grow with the screen because a card is a picture of an object and a
 * bigger screen should show a bigger object. Text does not work that way: a
 * 12px panel title on a laptop is 12px on a monitor because that is how big
 * readable text is, and blowing it up to 19 would make the board shout its
 * own furniture at the player. So above scale 1 the chrome takes a fraction
 * of the growth — enough that it does not look starved beside a card half
 * again as large, nowhere near enough to keep pace with it.
 *
 * Below 1 they move together and this does nothing: on a phone, everything
 * shrinking together is exactly right, and it is what shipped.
 */
function chromeScale(scale: number): number {
  if (scale <= 1) return scale;
  // And a ceiling on top of the fraction, because the fraction alone stopped
  // being enough once cards could reach twice the baseline: two-fifths of
  // *that* growth puts a 12px panel title at 17, and a 17px title is the
  // board's furniture talking over the game. 1.33 is the largest that keeps
  // it at 16.
  return Math.min(1.33, 1 + (scale - 1) * 0.4);
}

function dim(n: number, scale: number): number {
  return Math.round(n * scale);
}

/** Never below 9 — a font that keeps shrinking with the card stops being legible before the card does. */
function font(n: number, scale: number): number {
  return Math.max(9, Math.round(n * scale));
}

export function metricsFor(width: number): Metrics {
  const scale = scaleFor(width);
  const chrome = chromeScale(scale);
  const narrow = width < 768;

  const card: CardMetrics = {
    width: dim(BASE_CARD.width, scale),
    height: dim(BASE_CARD.height, scale),
    gap: dim(BASE_CARD.gap, scale),
    ringPadding: BASE_CARD.ringPadding,
    ringBorder: BASE_CARD.ringBorder,
    compactWidth: dim(BASE_CARD.compactWidth, scale),
    compactHeight: dim(BASE_CARD.compactHeight, scale),
    rankFont: font(BASE_CARD.rankFont, scale),
    suitFont: font(BASE_CARD.suitFont, scale),
    jokerRankFont: font(BASE_CARD.jokerRankFont, scale),
    suitInlineFont: font(BASE_CARD.suitInlineFont, scale),
    fanGap: BASE_CARD.fanGap,
    // Set below, once the parts above are known.
    slotPitch: 0,
  };
  card.slotPitch =
    card.width +
    card.gap +
    // The selection ring CardView draws around every card...
    2 * (card.ringPadding + card.ringBorder) +
    // ...the slot HandZone wraps that ring in, whose border is always there
    // so that opening a drop gap never moves anything...
    2 * card.ringBorder +
    // ...and the row's own gap to the next slot.
    card.fanGap;

  const seat: SeatMetrics = {
    avatar: dim(BASE_SEAT.avatar, scale),
    avatarCompact: dim(BASE_SEAT.avatarCompact, scale),
  };

  const panel: PanelMetrics = {
    padding: dim(BASE_PANEL.padding, chrome),
    gap: dim(BASE_PANEL.gap, chrome),
    radius: BASE_PANEL.radius,
    titleFont: font(BASE_PANEL.titleFont, chrome),
    bodyFont: font(BASE_PANEL.bodyFont, chrome),
  };

  return {
    scale,
    narrow,
    roomy: scale > 1,
    maxWidth: Math.min(width, BOARD_MAX_WIDTH),
    card,
    panel,
    seat,
    stackedCorner: Math.max(20, dim(BASE_STACKED_CORNER, scale)),
    buttonMinWidth: Math.max(64, dim(BASE_BUTTON_MIN_WIDTH, chrome)),
  };
}

/**
 * The padding the match screen keeps outside the board, on each side.
 *
 * Named here because the hand's own arithmetic needs it (see `handRowWidth`)
 * and because it was already a bare 16 written twice — once in the screen and
 * once in the test that checks a thirteen-card hand fits. A number two places
 * hold separately is how the scale ceiling got set wrong the first time.
 */
export const SCREEN_PADDING = 16;

/**
 * How much width the hand's row of cards has to lay out in, as far as the
 * metrics can tell from the window.
 *
 * An estimate, and knowingly a generous one: on the web the window width
 * includes a scrollbar the layout does not get, and the board carries padding
 * this cannot see from here. Good enough to decide how big a card may be —
 * which is all it was ever used for — and good enough as a first guess before
 * the row has measured itself. `fanPitch` takes the measured width instead
 * once there is one, because being fifteen pixels optimistic there is the
 * difference between a hand in one row and a hand in two.
 */
export function handRowWidth(m: Metrics): number {
  return m.maxWidth - 2 * SCREEN_PADDING - 2 * m.panel.padding;
}

/**
 * The least of a card that still says which card it is.
 *
 * The engraved deck prints its index in the top-left corner, so a card
 * overlapped from the right is identified by the strip down its left edge —
 * exactly the way a hand of cards fanned between two fingers is. This is how
 * wide that strip is allowed to get down to: enough for the rank, the suit
 * under it, and a little air.
 */
export function minPeek(m: Metrics): number {
  // Wide enough for the widest index any face draws — "10" set bold, with a
  // suit under it. The engraved deck keeps its index inside the left fifth of
  // the card, so this is set by the drawn faces rather than by that one.
  return Math.max(20, Math.round(m.card.width * 0.38));
}

/**
 * The least of a card that still says a card is *there*, while one is being
 * carried out of the hand.
 *
 * A closed hand sits exactly on `minPeek` — that is what `fanPitch`'s closed
 * branch returns — so it has nothing left to give, and a hand that fills its
 * row has no empty felt to give either. Between them that is every closed
 * hand on a small screen, which is precisely where a card-shaped hole for a
 * dropped card was impossible to open.
 *
 * So for the few seconds a card is out of the hand, and only then, the rest of
 * the fan may close up past the point where you could read it. A hand with a
 * card lifted out of it is being aimed at rather than read; the indices come
 * back the instant the card is let go. Seven tenths is enough to buy a whole
 * card at every width a hand is dealt at, and taken *from* `minPeek` rather
 * than from a second fraction of a card, so the two cannot drift apart and
 * this one is always the smaller.
 */
export function dragPeek(m: Metrics): number {
  return Math.max(12, Math.round(minPeek(m) * 0.7));
}

/**
 * How far apart the cards in a hand sit.
 *
 * A hand is held, not laid out. Past the point where the cards would fit side
 * by side they slide over each other instead — each covering the one before
 * it, all of them still in one row, every one still showing the corner that
 * names it. That is what a hand of cards looks like in a hand, and it is the
 * default here rather than a fallback: wrapping a hand onto a second and
 * third row costs the vertical space the board has least of, and means the
 * hand you are hunting a card in is a different shape every turn.
 *
 * Two pitches, and nothing in between:
 *
 *  - **Full**, when the whole hand fits that way. A hand of three has no
 *    reason to hug itself into a corner of a board with room to spare.
 *  - **Closed**, when it does not: every card down to the strip that names
 *    it, stacked as tightly as the deck can still be read.
 *
 * Not "as much overlap as it takes to fit", which was the first cut of this
 * and looked like a bug — the hand tightened by a pixel or two per card as
 * cards arrived, so it was never quite a row and never quite a fan. It closes
 * all the way or not at all, and `spread` is the player's own override.
 */
export function fanPitch(
  m: Metrics,
  count: number,
  measuredWidth?: number,
  spread?: boolean,
): number {
  const full = m.card.slotPitch;
  if (count < 2) return full;
  // What one slot takes on screen: the pitch, less the row gap that trails it.
  const slot = full - m.card.fanGap;
  const room =
    measuredWidth && measuredWidth > 0 ? Math.floor(measuredWidth) : handRowWidth(m);
  if ((count - 1) * full + slot <= room) return full;
  // Opened out: as wide as the row will take and no wider. Not `full`, which
  // would put the hand back onto two rows — the player asked to see the cards,
  // not to get the wrapping back.
  if (spread) {
    return Math.max(minPeek(m), Math.min(full, Math.floor((room - slot) / (count - 1))));
  }
  const closed = minPeek(m);
  // A hand long enough to overflow even closed is past anything a deal
  // produces; it wraps, at the tightest pitch, rather than tightening further
  // into an unreadable stack.
  return Math.min(full, closed);
}

/** Whether `fanPitch` is closing the hand up rather than laying it out. */
export function fanOverlaps(
  m: Metrics,
  count: number,
  measuredWidth?: number,
  spread?: boolean,
): boolean {
  return fanPitch(m, count, measuredWidth, spread) < m.card.slotPitch;
}
