import { SKINS } from '@/src/skins';
import { classic } from '@/src/skins/classic';
import { contrast as highContrast } from '@/src/skins/contrast';
import { contrast, parseColor } from '@/src/skins/contrastRatio';
import type { Skin, SkinColors } from '@/src/skins/types';
import { colors as staticColors, highContrastColors } from '@/src/theme';

/**
 * Every skin's palette, held to WCAG 2.2 contrast (docs/accessibility-plan.md,
 * Phases 1, 5 and 6).
 *
 * The rule is checked here, per token, rather than by an axe run per screen:
 * a token that fails fails everywhere it is used, and a screen scan only
 * finds the places somebody happened to visit. The thresholds:
 *
 *  - 4.5:1 for text (1.4.3), on every surface it can sit on;
 *  - 3:1 for the edges that identify a component or a state — the drop
 *    target, the selection ring, a card against the felt (1.4.11);
 *  - 7:1 for every text colour of the high-contrast skin (1.4.6), which is
 *    the whole of what that skin promises.
 *
 * Surfaces are composited, not taken at face value: a panel on Casino is
 * 66%-opaque glass over a felt that runs from dark to lit under a lamp, so
 * every text colour is measured on the glass over each of the felt's stops,
 * with the lamp's darkest and lightest wash over them too. A failure names
 * the skin, the token and the surface, so it can be fixed at the token.
 */

type Check = { what: string; ratio: number; min: number };

/**
 * The felt: each gradient stop, bare and under the lamp's darkened rim and
 * the side edges — and, with `lit`, under the lamp's bright centre and the
 * sheen too.
 */
function grounds(skin: Skin, lit = true): { name: string; stack: string[] }[] {
  const out: { name: string; stack: string[] }[] = [{ name: 'bg', stack: [skin.colors.bg] }];
  skin.table.background.forEach((stop, i) => {
    out.push({ name: `felt[${i}]`, stack: [stop] });
    if (skin.table.lamp) {
      out.push({ name: `felt[${i}]+lamp.outer`, stack: [skin.table.lamp.outer, stop] });
      if (lit) out.push({ name: `felt[${i}]+lamp.inner`, stack: [skin.table.lamp.inner, stop] });
    }
    if (lit && skin.table.sheen) out.push({ name: `felt[${i}]+sheen`, stack: [skin.table.sheen, stop] });
    if (skin.table.edge) out.push({ name: `felt[${i}]+edge`, stack: [skin.table.edge, stop] });
  });
  return out;
}

/**
 * Where words are set: on a panel or the surface colour over any part of the
 * felt, and on the bare felt itself.
 *
 * Bare-felt words are measured on the felt's stops and its lamp-darkened rim
 * but not under the lamp's bright centre: the words set straight on the felt
 * are the board's edges — the header, the rules link, the look switcher, the
 * status lines under the controls — and the lit middle of the table is where
 * the panels sit, which *are* measured there. An approximation, said here
 * rather than hidden: the lamp is a radial gradient and this samples its ends.
 */
function textSurfaces(skin: Skin): { name: string; stack: string[] }[] {
  const out: { name: string; stack: string[] }[] = [...grounds(skin, false)];
  for (const g of grounds(skin)) {
    out.push({ name: `panel on ${g.name}`, stack: [skin.panel.background, ...g.stack] });
    out.push({ name: `surface on ${g.name}`, stack: [skin.colors.surface, ...g.stack] });
  }
  return out;
}

/** What a card's type can be printed on: plain stock, the wash's two ends, a selected card. */
function cardStocks(skin: Skin): { name: string; stack: string[] }[] {
  const out = [
    { name: 'cardBg', stack: [skin.colors.cardBg] },
    { name: 'selectedFace', stack: [skin.card.selectedFace] },
  ];
  skin.card.faceGradient?.forEach((c, i) => out.push({ name: `faceGradient[${i}]`, stack: [c] }));
  return out;
}

function skinChecks(skin: Skin): Check[] {
  const c = skin.colors;
  const hc = skin.id === highContrast.id;
  const textMin = hc ? 7 : 4.5;
  const checks: Check[] = [];

  // Text tokens. `text` and `muted` carry words everywhere; the rest carry
  // words in places — a link in accent, an error in danger, a picked option
  // in gold — so they are held to the text threshold too.
  const textTokens: (keyof SkinColors)[] = ['text', 'muted', 'accent', 'danger', 'success', 'gold'];
  for (const s of textSurfaces(skin)) {
    for (const tok of textTokens) {
      checks.push({ what: `${skin.id}: ${tok} on ${s.name}`, ratio: contrast(c[tok], ...s.stack), min: textMin });
    }
  }
  // Button labels, on the button.
  checks.push({ what: `${skin.id}: onAccent on accentButton`, ratio: contrast(c.onAccent, c.accentButton), min: textMin });
  // A chosen option's label sits on accentDim.
  checks.push({ what: `${skin.id}: text on accentDim`, ratio: contrast(c.text, c.accentDim), min: textMin });

  // Card type: the two inks, and the four-colour deck's three, on every stock.
  const inks: [string, string][] = [
    ['card.ink', skin.card.ink],
    ['card.red', skin.card.red],
    ['fourColour.diamonds', skin.card.fourColour.diamonds],
    ['fourColour.clubs', skin.card.fourColour.clubs],
    ['fourColour.bells', skin.card.fourColour.bells],
  ];
  for (const s of cardStocks(skin)) {
    for (const [name, ink] of inks) {
      checks.push({ what: `${skin.id}: ${name} on ${s.name}`, ratio: contrast(ink, ...s.stack), min: textMin });
    }
  }
  // A joker's "JKR" is printed in red on the joker's own fill.
  checks.push({ what: `${skin.id}: card.red on jokerFace`, ratio: contrast(skin.card.red, skin.card.jokerFace), min: textMin });
  // The engraved deck's own inks, on its own stock.
  if (skin.card.cardPalette) {
    const p = skin.card.cardPalette;
    for (const [name, ink] of [
      ['cardPalette.ink', p.ink],
      ['cardPalette.red', p.red],
    ] as const) {
      checks.push({ what: `${skin.id}: ${name} on cardPalette.stock`, ratio: contrast(ink, p.stock), min: textMin });
    }
  }

  // Non-text: what tells a component or a state apart from what is around it.
  for (const g of grounds(skin)) {
    checks.push({ what: `${skin.id}: card (cardBg) on ${g.name}`, ratio: contrast(c.cardBg, ...g.stack), min: 3 });
    checks.push({ what: `${skin.id}: drop target border on ${g.name}`, ratio: contrast(skin.dropArmed.borderColor, ...g.stack), min: 3 });
    checks.push({ what: `${skin.id}: accent (live target) on ${g.name}`, ratio: contrast(c.accent, ...g.stack), min: 3 });
    checks.push({ what: `${skin.id}: gold (selection ring) on ${g.name}`, ratio: contrast(c.gold, ...g.stack), min: 3 });
  }

  if (hc) {
    // The high-contrast skin's promise: solid, stated edges.
    for (const g of grounds(skin)) {
      checks.push({ what: `${skin.id}: panel.border on ${g.name}`, ratio: contrast(skin.panel.border, ...g.stack), min: 3 });
      checks.push({ what: `${skin.id}: colors.border on ${g.name}`, ratio: contrast(c.border, ...g.stack), min: 3 });
    }
    checks.push({ what: `${skin.id}: cardBorder on cardBg`, ratio: contrast(c.cardBorder, c.cardBg), min: 3 });
  }
  return checks;
}

function failures(checks: Check[]): string[] {
  return checks.filter((k) => k.ratio < k.min).map((k) => `${k.what}: ${k.ratio.toFixed(2)} < ${k.min}`);
}

describe('skin contrast', () => {
  for (const skin of SKINS) {
    it(`${skin.id} meets its contrast thresholds`, () => {
      expect(failures(skinChecks(skin))).toEqual([]);
    });
  }

  it('the focus ring (white inside, black outside) shows on every surface of every skin', () => {
    // A11yRoot draws one ring everywhere; one of its two colours always
    // clears 3:1, which is why it is two. Checked so a future single-colour
    // ring has to answer to this.
    const bad: string[] = [];
    for (const skin of SKINS) {
      for (const s of [...textSurfaces(skin), ...cardStocks(skin)]) {
        const best = Math.max(contrast('#ffffff', ...s.stack), contrast('#000000', ...s.stack));
        if (best < 3) bad.push(`${skin.id}: ${s.name} ${best.toFixed(2)}`);
      }
    }
    expect(bad).toEqual([]);
  });

  it('the high-contrast skin has no texture or gradient', () => {
    expect(highContrast.table.background).toHaveLength(1);
    expect(highContrast.table.lamp).toBeUndefined();
    expect(highContrast.table.sheen).toBeUndefined();
    expect(highContrast.table.edge).toBeUndefined();
    expect(highContrast.card.faceGradient).toBeUndefined();
    expect(highContrast.card.bevel).toBeUndefined();
    expect(highContrast.panel.bevel).toBeUndefined();
    expect(highContrast.card.back.colors[0]).toBe(highContrast.card.back.colors[1]);
    // Every colour it declares is opaque: a translucent colour's contrast
    // depends on what is under it, and this skin promises it does not.
    const opaque = (c: string) => parseColor(c)[3] === 1;
    for (const v of Object.values(highContrast.colors)) expect(opaque(v)).toBe(true);
    expect(opaque(highContrast.panel.background)).toBe(true);
    expect(opaque(highContrast.panel.border)).toBe(true);
  });
});

describe('static palette (screens outside a match)', () => {
  const check = (name: string, c: SkinColors, textMin: number) => {
    const out: Check[] = [];
    for (const [sname, bg] of [
      ['bg', [c.bg]],
      ['surface', [c.surface, c.bg]],
    ] as const) {
      for (const tok of ['text', 'muted'] as const) {
        out.push({ what: `${name}: ${tok} on ${sname}`, ratio: contrast(c[tok], ...bg), min: textMin });
      }
      for (const tok of ['accent', 'danger', 'success', 'gold'] as const) {
        out.push({ what: `${name}: ${tok} on ${sname}`, ratio: contrast(c[tok], ...bg), min: 4.5 });
      }
    }
    out.push({ what: `${name}: onAccent on accentButton`, ratio: contrast(c.onAccent, c.accentButton), min: 4.5 });
    return out;
  };

  it('is the classic palette by default', () => {
    // No stored preference and no system contrast in the test environment.
    expect(staticColors).toBe(classic.colors);
  });

  it('meets AA', () => {
    expect(failures(check('theme', classic.colors, 4.5))).toEqual([]);
  });

  it('meets 7:1 for words, and 3:1 for edges, when high contrast was asked for', () => {
    const c = highContrastColors;
    const checks = check('theme (high contrast)', c, 7);
    checks.push({ what: 'theme (high contrast): border on surface', ratio: contrast(c.border, c.surface, c.bg), min: 3 });
    checks.push({ what: 'theme (high contrast): border on bg', ratio: contrast(c.border, c.bg), min: 3 });
    expect(failures(checks)).toEqual([]);
  });
});
