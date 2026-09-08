import type { MatchModule } from '@/src/api/matchTypes';
import type { TallyView } from '@/src/api/types';
import {
  aiLabel,
  avgRankText,
  difficultySplits,
  moduleLabel,
  played,
  playerCountLabel,
  playerCountSplits,
  plural,
  streakText,
  subjectName,
  winPercent,
  winPercentText,
} from '@/src/lib/stats';

function tally(over: Partial<TallyView> = {}): TallyView {
  return {
    matches: 0,
    wins: 0,
    losses: 0,
    draws: 0,
    scoreSum: 0,
    rankSum: 0,
    winRate: 0,
    avgScore: 0,
    avgRank: 0,
    bestScore: null,
    worstScore: null,
    ...over,
  };
}

describe('played', () => {
  it('is false for an untouched bucket and for a missing one', () => {
    expect(played(tally())).toBe(false);
    expect(played(undefined)).toBe(false);
  });

  it('is true as soon as one match is in it, however it went', () => {
    expect(played(tally({ matches: 1, losses: 1 }))).toBe(true);
  });
});

describe('winPercent', () => {
  it('rounds the rate to a whole percent', () => {
    expect(winPercent(tally({ matches: 3, wins: 1, winRate: 1 / 3 }))).toBe(33);
    expect(winPercent(tally({ matches: 8, wins: 5, winRate: 0.625 }))).toBe(63);
  });

  it('is null, not zero, when nothing has been played', () => {
    // The distinction the whole module rests on: a player who has never sat at
    // this kind of table has not lost every one of them.
    expect(winPercent(tally())).toBeNull();
    expect(winPercentText(tally())).toBe('—');
  });

  it('reads 0% only when matches were actually played and lost', () => {
    expect(winPercentText(tally({ matches: 4, losses: 4 }))).toBe('0%');
  });
});

describe('streakText', () => {
  it('reads a positive streak as wins and a negative one as losses', () => {
    expect(streakText(4)).toBe('4 wins in a row');
    expect(streakText(-3)).toBe('3 losses in a row');
  });

  it('does not say "1 wins in a row"', () => {
    expect(streakText(1)).toBe('Won the last one');
    expect(streakText(-1)).toBe('Lost the last one');
  });

  it('treats zero — a draw, or no matches — as no streak', () => {
    expect(streakText(0)).toBe('No streak');
  });
});

describe('avgRankText', () => {
  it('shows one decimal, since the interesting differences are fractional', () => {
    expect(avgRankText(tally({ matches: 3, avgRank: 2.333 }))).toBe('2.3');
  });

  it('has nothing to average before a match is played', () => {
    expect(avgRankText(tally())).toBe('—');
  });
});

describe('plural', () => {
  it('picks the noun by the count', () => {
    expect(plural(1, 'player')).toBe('1 player');
    expect(plural(4, 'player')).toBe('4 players');
    expect(plural(0, 'player')).toBe('0 players');
  });
});

describe('moduleLabel', () => {
  const modules = [
    { id: 'zolik', label: 'Žolíky' },
    { id: 'canasta', label: 'Canasta' },
  ] as MatchModule[];

  it('uses the module list the lobby already has', () => {
    expect(moduleLabel('zolik', modules)).toBe('Žolíky');
  });

  it('humanises an id no longer in the list rather than dropping the row', () => {
    // A module can be unregistered after matches were recorded under it. Those
    // matches still happened, so the row survives without its label.
    expect(moduleLabel('ginRummy', modules)).toBe('Gin rummy');
  });
});

describe('playerCountLabel', () => {
  it('turns the count key into a table size', () => {
    expect(playerCountLabel('2')).toBe('2 players');
    expect(playerCountLabel('1')).toBe('1 player');
  });

  it('falls back for a key that is not a count', () => {
    expect(playerCountLabel('unknown')).toBe('Unknown');
  });
});

describe('aiLabel', () => {
  it('reads a persona key as a name and a strength', () => {
    expect(aiLabel('hard:miroslav')).toBe('Miroslav (hard)');
  });

  it('reads a bare difficulty — a bot seated before personas existed', () => {
    expect(aiLabel('expert')).toBe('Expert');
    expect(aiLabel('unspecified')).toBe('Unspecified');
  });
});

describe('subjectName', () => {
  it('prefers the recorded display name', () => {
    expect(subjectName({ kind: 'user', id: 'abc', name: 'Dana' })).toBe('Dana');
  });

  it('never shows a raw account id as if it were a name', () => {
    expect(subjectName({ kind: 'user', id: '65f1c0de0000000000000000', name: '' })).toBe(
      'Unknown player',
    );
  });

  it('falls back to a bot persona, which is meaningful on its own', () => {
    expect(subjectName({ kind: 'ai', id: 'medium:petra', name: '' })).toBe('Petra (medium)');
  });
});

describe('difficultySplits', () => {
  it('orders weakest first and puts unknown strengths last', () => {
    const got = difficultySplits({
      'hard:miroslav': tally({ matches: 2 }),
      easy: tally({ matches: 1 }),
      unspecified: tally({ matches: 1 }),
      'medium:petra': tally({ matches: 3 }),
    });
    expect(got.map((s) => s.key)).toEqual(['easy', 'medium:petra', 'hard:miroslav', 'unspecified']);
  });

  it('drops buckets nothing was played in', () => {
    const got = difficultySplits({ easy: tally({ matches: 1 }), expert: tally() });
    expect(got.map((s) => s.key)).toEqual(['easy']);
  });

  it('handles the split being absent entirely', () => {
    expect(difficultySplits(undefined)).toEqual([]);
  });
});

describe('playerCountSplits', () => {
  it('orders by table size numerically, not as strings', () => {
    const got = playerCountSplits({
      '10': tally({ matches: 1 }),
      '2': tally({ matches: 1 }),
      '4': tally({ matches: 1 }),
    });
    expect(got.map((s) => s.key)).toEqual(['2', '4', '10']);
    expect(got[0].label).toBe('2 players');
  });
});
