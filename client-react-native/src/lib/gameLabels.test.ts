import {
  choiceLabel,
  moduleLabel,
  moduleName,
  optionHelp,
  optionLabel,
  variationLabel,
  variationName,
} from '@/src/lib/gameLabels';

// What matters here is the fallback chain, not any one translation: a game,
// option or choice the app has never heard of must read as the English the
// server sent, never as a raw key, so a server ahead of the app stays usable.
describe('gameLabels fall back to what the server sent', () => {
  it('a module this build has no words for is shown by the server label, or failing that its id', () => {
    expect(moduleLabel({ id: 'brand-new-game', label: 'Brand New' })).toBe('Brand New');
    expect(moduleName('brand-new-game')).toBe('brand-new-game');
  });

  it('a variation, option, help line and choice nobody has worded are the server label', () => {
    expect(variationLabel('brand-new-game', { id: 'wild', label: 'Wild rules' })).toBe('Wild rules');
    expect(variationName('brand-new-game', 'wild')).toBe('wild');
    expect(optionLabel('brand-new-game', { name: 'zzNoSuchOption', label: 'Zz option' })).toBe('Zz option');
    expect(optionHelp('brand-new-game', { name: 'zzNoSuchOption', help: 'Does a thing.' })).toBe('Does a thing.');
    expect(choiceLabel('brand-new-game', 'zzNoSuchOption', { value: 7, label: 'Seven' })).toBe('Seven');
  });

  it('an option with no help gives no tooltip rather than an empty one', () => {
    expect(optionHelp('prsi', { name: 'anything' })).toBeUndefined();
    expect(optionHelp('prsi', { name: 'anything', help: '' })).toBeUndefined();
  });

  it('a game that has a worded name uses it rather than the server English', () => {
    // Most game names are the same in every language and are deliberately left
    // to the server's label; the ones that carry a diacritic or a translation
    // are keyed (module.klondike reads "Solitaire" whatever the server says).
    expect(moduleLabel({ id: 'klondike', label: 'Klondike' })).toBe('Solitaire');
  });
});
