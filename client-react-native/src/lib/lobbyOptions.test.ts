import type { OptionSpec, ProfileSpec, ResolvedRules } from '@/src/api/types';
import {
  defaultsFor,
  labelFor,
  lastOptionKey,
  nextChoice,
  restoreChoice,
} from '@/src/lib/lobbyOptions';

// These helpers exist so the lobby renders whatever the server declares. The
// tests therefore use option names and values the client has no knowledge of
// wherever possible — if a test only passes for `initialMeldMinimum`, the
// helper is not generic and the duplication has crept back.

const meldFloor: OptionSpec = {
  name: 'initialMeldMinimum',
  type: 'enum_int',
  label: 'Meld value',
  choices: [
    { value: 0, label: 'Off' },
    { value: 35, label: '35' },
    { value: 50, label: '50' },
  ],
};

// An option this client has never heard of, to prove the helpers are driven
// by data rather than by names they recognise.
const madeUpKnob: OptionSpec = {
  name: 'wildcardCount',
  type: 'enum_int',
  label: 'Wildcards',
  choices: [
    { value: 2, label: 'Two' },
    { value: 4, label: 'Four' },
  ],
};

const rules = (over: Partial<ResolvedRules>): ResolvedRules => ({
  profile: 'made_up',
  dealSize: 13,
  minSetSize: 3,
  minRunSize: 3,
  initialMeldMinimum: 0,
  discardDrawMinRound: 0,
  discardPickupMode: 'any_from_pile',
  jokerDiscardRestricted: true,
  fixedDealCount: 0,
  matchEndMode: 'at_score',
  targetScore: 200,
  ...over,
});

const profile = (over: Partial<ResolvedRules>): ProfileSpec => ({
  id: 'made_up',
  label: 'Made Up',
  rules: rules(over),
  contract: { sets: 0, runs: 0, requireCleanRun: false },
});

describe('defaultsFor', () => {
  it('reads each option off the profile ruleset by the option own name', () => {
    const got = defaultsFor(profile({ initialMeldMinimum: 35, discardDrawMinRound: 3 }), [
      meldFloor,
      { ...meldFloor, name: 'discardDrawMinRound' },
    ]);
    expect(got).toEqual({ initialMeldMinimum: 35, discardDrawMinRound: 3 });
  });

  it('skips an option the ruleset has no value for', () => {
    // A server that declares a knob its profiles do not carry should leave the
    // chip blank rather than inventing a 0 that was never chosen.
    expect(defaultsFor(profile({}), [madeUpKnob])).toEqual({});
  });

  it('returns nothing when no profile is selected yet', () => {
    expect(defaultsFor(undefined, [meldFloor])).toEqual({});
  });
});

describe('labelFor', () => {
  it('uses the label the server sent', () => {
    expect(labelFor(meldFloor, 0)).toBe('Off');
    expect(labelFor(meldFloor, 35)).toBe('35');
    expect(labelFor(madeUpKnob, 4)).toBe('Four');
  });

  it('shows the raw value rather than nothing for an unlabelled choice', () => {
    // A newer server offering a value this build has no label for must still
    // display something truthful.
    expect(labelFor(meldFloor, 70)).toBe('70');
  });

  it('marks an unset value instead of rendering undefined', () => {
    expect(labelFor(meldFloor, undefined)).toBe('—');
  });
});

describe('nextChoice', () => {
  it('cycles through the declared choices and wraps', () => {
    expect(nextChoice(meldFloor, 0)).toBe(35);
    expect(nextChoice(meldFloor, 35)).toBe(50);
    expect(nextChoice(meldFloor, 50)).toBe(0);
  });

  it('cycles an option this client has never heard of', () => {
    expect(nextChoice(madeUpKnob, 2)).toBe(4);
    expect(nextChoice(madeUpKnob, 4)).toBe(2);
  });

  it('starts from the beginning for a value no longer on the list', () => {
    // A retired setting must not leave the chip stuck: tapping it has to move.
    expect(nextChoice(meldFloor, 70)).toBe(0);
    expect(nextChoice(meldFloor, undefined)).toBe(0);
  });
});

describe('restoreChoice', () => {
  it('restores a stored value the server still declares', () => {
    expect(restoreChoice(meldFloor, '35')).toBe(35);
  });

  it('refuses a stored value the server has retired', () => {
    // The descriptor is the authority, not this device's cache — otherwise a
    // removed option resurrects itself and the create call gets a 400.
    expect(restoreChoice(meldFloor, '70')).toBeUndefined();
  });

  it('ignores junk and absence', () => {
    expect(restoreChoice(meldFloor, null)).toBeUndefined();
    expect(restoreChoice(meldFloor, 'not a number')).toBeUndefined();
  });
});

describe('lastOptionKey', () => {
  it('derives a distinct key per option', () => {
    expect(lastOptionKey('initialMeldMinimum')).not.toBe(lastOptionKey('discardDrawMinRound'));
    expect(lastOptionKey('wildcardCount')).toContain('wildcardCount');
  });
});

describe('the no-rule-knowledge invariant', () => {
  const source = require('fs').readFileSync(require.resolve('@/src/lib/lobbyOptions'), 'utf8');
  const body = source
    .split('\n')
    .filter(
      (l: string) =>
        !l.trim().startsWith('*') && !l.trim().startsWith('//') && !l.trim().startsWith('/*'),
    )
    .join('\n');

  it.each([
    ['a profile name', /zolik_classic|continental/],
    ['a meld-minimum constant', /\b35\b|\b50\b|\b70\b/],
    ['a lock-round constant', /discardDrawMinRound|initialMeldMinimum/],
  ])('contains no %s', (_label, pattern) => {
    expect(body).not.toMatch(pattern as RegExp);
  });
});
