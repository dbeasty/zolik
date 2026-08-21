import type { ResolvedRules } from '@/src/api/types';
import { contractLabel, dealHeaderLabel, rulesSummaryLines } from '@/src/lib/cards';

// These helpers used to switch on `rulesProfile === 'zolik_classic'` and
// carry their own copies of server/internal/rules/profiles.go, so a profile
// the client had never heard of rendered some other profile's numbers. Now
// every value comes off the ruleset the server resolved for this game.

const continental: ResolvedRules = {
  profile: 'continental',
  dealSize: 12,
  minSetSize: 3,
  minRunSize: 4,
  initialMeldMinimum: 35,
  discardDrawMinRound: 3,
  discardPickupMode: 'top_only',
  jokerDiscardRestricted: true,
  fixedDealCount: 7,
  matchEndMode: 'after_deals',
  targetScore: 0,
};

const zolik: ResolvedRules = {
  profile: 'zolik_classic',
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
};

function panel(rules: ResolvedRules | undefined, contract: Parameters<typeof rulesSummaryLines>[1]) {
  return Object.fromEntries(rulesSummaryLines(rules, contract).map((l) => [l.label, l.value]));
}

describe('contractLabel', () => {
  it('names the contract the server sent, not one looked up by deal number', () => {
    expect(contractLabel({ sets: 2, runs: 0, requireCleanRun: false })).toBe('Two sets');
    expect(contractLabel({ sets: 1, runs: 1, requireCleanRun: false })).toBe('One set, One run');
    expect(contractLabel({ sets: 0, runs: 3, requireCleanRun: false })).toBe('Three runs');
  });

  it('describes a shapeless contract by its clean-run clause', () => {
    expect(contractLabel({ sets: 0, runs: 0, requireCleanRun: true })).toMatch(/joker-free/);
    expect(contractLabel({ sets: 0, runs: 0, requireCleanRun: false })).toBe('Any valid meld');
  });

  it('appends the clean-run clause to a counted contract', () => {
    expect(contractLabel({ sets: 1, runs: 1, requireCleanRun: true })).toBe(
      'One set, One run — one run must be joker-free',
    );
  });
});

describe('dealHeaderLabel', () => {
  it('counts deals only for a fixed-length match', () => {
    expect(dealHeaderLabel(continental, { sets: 2, runs: 0, requireCleanRun: false }, 1)).toBe(
      'Game 1 of 7: Two sets',
    );
  });

  it('does not invent a deal count for a score-limited match', () => {
    // The bug this replaces: a hardcoded "of 7" that read "Game 9 of 7"
    // once a score-limited match ran past the seventh deal.
    expect(dealHeaderLabel(zolik, { sets: 0, runs: 0, requireCleanRun: true }, 9)).toBe('Deal 9');
  });
});

describe('rulesSummaryLines', () => {
  it('reports continental constants', () => {
    const c = panel(continental, { sets: 2, runs: 0, requireCleanRun: false });
    expect(c['Deal size']).toBe('12 cards');
    expect(c['Minimum run length']).toBe('4 cards');
    expect(c['Discard pickup']).toBe('Top card only, locked until round 3');
    expect(c['Match ends']).toBe('After 7 deals');
  });

  it('reports zolik_classic constants', () => {
    const z = panel(zolik, { sets: 0, runs: 0, requireCleanRun: true });
    expect(z['Deal size']).toBe('13 cards');
    expect(z['Minimum run length']).toBe('3 cards');
    expect(z['Discard pickup']).toMatch(/Any card in the pile/);
    expect(z['Meld value floor']).toBe('Off');
    expect(z['Match ends']).toBe('First to 200 points');
  });

  it('renders a profile it has never seen, from its values alone', () => {
    // The actual extensibility claim: adding a third profile server-side
    // needs no client change. No name below is one this code knows.
    const houseRules: ResolvedRules = {
      profile: 'burraco_house',
      dealSize: 11,
      minSetSize: 3,
      minRunSize: 3,
      initialMeldMinimum: 50,
      discardDrawMinRound: 0,
      discardPickupMode: 'any_from_pile',
      jokerDiscardRestricted: false,
      fixedDealCount: 0,
      matchEndMode: 'at_score',
      targetScore: 500,
    };
    const lines = panel(houseRules, { sets: 1, runs: 2, requireCleanRun: false });
    expect(lines['Deal size']).toBe('11 cards');
    expect(lines['Meld value floor']).toBe('50+ points on your first meld(s)');
    expect(lines['To go down']).toBe('One set, Two runs');
    expect(lines['Jokers']).toBe('Can be discarded freely');
    expect(lines['Match ends']).toBe('First to 500 points');
    expect(dealHeaderLabel(houseRules, { sets: 1, runs: 2, requireCleanRun: false }, 4)).toBe(
      'Deal 4',
    );
  });

  it('renders nothing rather than guessing when the server sent no ruleset', () => {
    expect(rulesSummaryLines(undefined, undefined)).toEqual([]);
    expect(dealHeaderLabel(undefined, undefined, 3)).toBe('Deal 3');
  });
});
