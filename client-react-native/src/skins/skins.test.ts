import { DEFAULT_SKIN, SKINS, defaultSkinFor, skinById } from '@/src/skins';

/**
 * Which board somebody who has never chosen a skin lands on.
 *
 * Worth a test rather than a glance because the rule is a *size* one and the
 * thing it picks is a *look* — the one place those two are allowed to meet,
 * and therefore the one place a careless edit could quietly hand a phone the
 * engraved deck it cannot draw legibly.
 */
describe('the default skin', () => {
  it('is the engraved board wherever there is room for it', () => {
    // A laptop, a monitor, and the width the layout starts growing cards at.
    for (const width of [768, 1024, 1280, 1600, 2560]) {
      expect(`${width}: ${defaultSkinFor(width).id}`).toBe(`${width}: heirloom`);
    }
  });

  it('is the one-index board on a phone', () => {
    // A small phone, a large one, and the last width before the layout stops
    // calling itself narrow.
    for (const width of [320, 375, 414, 767]) {
      expect(`${width}: ${defaultSkinFor(width).id}`).toBe(`${width}: classic`);
    }
  });

  // The face a phone gets is the one drawn for a card that small: a single
  // index, sized to the strip a fanned hand shows. Said here because the
  // *reason* the phone default moved is the face, not the palette — a skin
  // swapped for a prettier one that drew three marks again would put the
  // phone back where it started.
  it('is a face with nothing on it but the index', () => {
    for (const width of [320, 375, 414, 767]) {
      expect(`${width}: ${defaultSkinFor(width).card.face}`).toBe(`${width}: plain`);
    }
  });

  it('turns on the same line the layout does, not a second one', () => {
    expect(defaultSkinFor(767).id).not.toBe(defaultSkinFor(768).id);
  });

  it('falls back to a skin that exists when no width is known', () => {
    expect(skinById(DEFAULT_SKIN.id)).toBe(DEFAULT_SKIN);
    expect(SKINS).toContain(DEFAULT_SKIN);
  });

  it('only ever picks a skin the switcher can cycle to', () => {
    for (const width of [320, 768, 1600]) expect(SKINS).toContain(defaultSkinFor(width));
  });
});
