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

const BASE_BUTTON_MIN_WIDTH = 92;
const BASE_STACKED_CORNER = 26;

/**
 * Breakpoints chosen against a real 375×812 phone viewport, where a 13-card
 * hand at scale 1 cost 407 of a 716px-tall screen. Below 768 the layout goes
 * to one column; below that, cards themselves start shrinking.
 *
 * Above 1280 they grow, and every step is the largest that still passes the
 * one thing that has to keep fitting: a thirteen-card hand — a full Žolíky
 * deal, the longest this client draws — in a single row, inside `maxWidth`
 * and its padding. A hand that wraps is a hand you have to re-find every
 * turn.
 *
 * Which is why growth starts at 1280 and not at 1024. Thirteen cards at scale
 * 1 already need about 930 of the 974 usable pixels a 1024 screen has, so
 * there is nothing to spend there: the first width that can afford a bigger
 * card is the first width with room for thirteen of them.
 *
 * The test checks this with the real `slotPitch` rather than by eye, which is
 * what caught the first cut of this — a 1.6 top step whose thirteenth card
 * wrapped onto a row of its own, because the arithmetic behind it had counted
 * the card and its gap and forgotten the slot border and the row gap.
 */
function scaleFor(width: number): number {
  if (width >= 1600) return 1.5;
  if (width >= 1280) return 1.3;
  if (width >= 768) return 1;
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
  return scale <= 1 ? scale : 1 + (scale - 1) * 0.4;
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
