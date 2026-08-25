/**
 * How big a card, a panel, or a control is, as a function of how wide the
 * screen is.
 *
 * One place decides this so a scaled card and its drag-and-drop drop gap
 * never disagree — see `HandZone`'s `DropGap`, which is built from the same
 * numbers a card is for exactly that reason. Everything below `narrow` is a
 * one-column phone layout; nothing at `scale: 1` differs from the sizes this
 * file replaces, so a desktop window is pixel-identical to before it existed.
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
};

export type PanelMetrics = {
  padding: number;
  gap: number;
  radius: number;
  titleFont: number;
  bodyFont: number;
};

export type Metrics = {
  /** 1 on a comfortable screen, less on a small one. */
  scale: number;
  /** A one-column screen: panels stack instead of sitting side by side. */
  narrow: boolean;
  card: CardMetrics;
  panel: PanelMetrics;
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
  ringPadding: 1,
  ringBorder: 2,
  compactWidth: 44,
  compactHeight: 60,
  rankFont: 14,
  suitFont: 18,
  jokerRankFont: 11,
  suitInlineFont: 14,
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
 */
function scaleFor(width: number): number {
  if (width >= 768) return 1;
  if (width >= 480) return 0.88;
  if (width >= 380) return 0.78;
  return 0.7;
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
  };

  const panel: PanelMetrics = {
    padding: dim(BASE_PANEL.padding, scale),
    gap: dim(BASE_PANEL.gap, scale),
    radius: BASE_PANEL.radius,
    titleFont: font(BASE_PANEL.titleFont, scale),
    bodyFont: font(BASE_PANEL.bodyFont, scale),
  };

  return {
    scale,
    narrow,
    card,
    panel,
    stackedCorner: Math.max(20, dim(BASE_STACKED_CORNER, scale)),
    buttonMinWidth: Math.max(64, dim(BASE_BUTTON_MIN_WIDTH, scale)),
  };
}
