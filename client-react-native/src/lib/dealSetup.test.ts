import type { MatchModule } from '@/src/api/matchTypes';

import { botChoices, botCount, dealWithBots, resolveSetup, seatRange } from './dealSetup';

// Pinochle's /modules entry as production serves it: two to four at the
// module, each variation pinned to its own table size.
const pinochle: MatchModule = {
  id: 'pinochle',
  label: 'Pinochle',
  minPlayers: 2,
  maxPlayers: 4,
  variations: [
    { id: 'partnership', label: 'Partnership', minPlayers: 4, maxPlayers: 4 },
    { id: 'cutthroat', label: 'Cutthroat', minPlayers: 3, maxPlayers: 3 },
    { id: 'twohanded', label: 'Two-handed', minPlayers: 2, maxPlayers: 2 },
  ],
};

// Canasta narrows only the top: Classic stops at four on a module that seats six.
const canasta: MatchModule = {
  id: 'canasta',
  label: 'Canasta',
  minPlayers: 2,
  maxPlayers: 6,
  defaultPlayers: 4,
  variations: [
    { id: 'classic', label: 'Classic', maxPlayers: 4 },
    { id: 'samba', label: 'Samba', maxPlayers: 6 },
  ],
};

const prsi: MatchModule = { id: 'prsi', label: 'Prší', minPlayers: 2, maxPlayers: 6 };

// Hold'em is six-handed, but its Draw variation names a table of its own.
const poker: MatchModule = {
  id: 'holdem',
  label: 'Poker',
  minPlayers: 2,
  maxPlayers: 9,
  defaultPlayers: 6,
  variations: [
    { id: 'holdem', label: 'Hold’em' },
    { id: 'draw', label: 'Five-Card Draw', maxPlayers: 6, defaultPlayers: 5 },
  ],
};

describe('seat range', () => {
  it('is the variation’s where it names one, the module’s otherwise', () => {
    expect(seatRange(pinochle, 'partnership')).toEqual({ min: 4, max: 4 });
    expect(seatRange(canasta, 'classic')).toEqual({ min: 2, max: 4 });
    expect(seatRange(prsi, undefined)).toEqual({ min: 2, max: 6 });
    expect(seatRange(pinochle, 'retired')).toEqual({ min: 2, max: 4 });
  });
});

describe('bot choices', () => {
  it('fill a Partnership Pinochle table, which needs four', () => {
    expect(botChoices(pinochle, 'partnership')).toEqual([3]);
    expect(botChoices(pinochle, 'cutthroat')).toEqual([2]);
    expect(botChoices(pinochle, 'twohanded')).toEqual([1]);
  });

  it('stop at Classic Canasta’s four, not the module’s six', () => {
    expect(botChoices(canasta, 'classic')).toEqual([1, 2, 3]);
    expect(botChoices(canasta, 'samba')).toEqual([1, 2, 3, 4, 5]);
  });

  it('are the module’s range for a game without narrowed variations', () => {
    expect(botChoices(prsi)).toEqual([1, 2, 3, 4, 5]);
  });
});

describe('bot count', () => {
  it('clamps a count saved under another variation into this one', () => {
    expect(botCount(pinochle, undefined, 'partnership')).toBe(3);
    expect(botCount(pinochle, 1, 'partnership')).toBe(3);
    expect(botCount(canasta, 5, 'classic')).toBe(3);
  });
});

describe('the default bot count', () => {
  it('fills the game’s usual table when nothing has been picked', () => {
    expect(botCount(canasta, undefined, 'classic')).toBe(3);
    expect(botCount(canasta, undefined, 'samba')).toBe(3);
    expect(botCount(poker, undefined, 'holdem')).toBe(5);
  });

  it('follows the variation, on the same screen, until a count is picked', () => {
    expect(botCount(poker, undefined, 'draw')).toBe(4);
    expect(botCount(pinochle, undefined, 'partnership')).toBe(3);
    expect(botCount(pinochle, undefined, 'cutthroat')).toBe(2);
    expect(botCount(pinochle, undefined, 'twohanded')).toBe(1);
  });

  it('gives way to what the player picked, even below the default', () => {
    expect(botCount(canasta, 1, 'classic')).toBe(1);
    expect(botCount(poker, 8, 'holdem')).toBe(8);
    expect(resolveSetup(canasta, { bots: 1 }).bots).toBe(1);
  });

  it('is the smallest table for a server that does not name one', () => {
    expect(botCount(prsi)).toBe(1);
  });
});

describe('Play now', () => {
  it('seats enough bots for the default variation to be dealt', async () => {
    const calls: string[] = [];
    const client = {
      createMatch: jest.fn(async () => ({ matchId: 'm1' })),
      addBot: jest.fn(async () => {
        calls.push('bot');
      }),
      startMatch: jest.fn(async () => {
        calls.push('start');
      }),
    };
    // What a game page's Play now sends with nothing saved.
    const setup = resolveSetup(pinochle, undefined);
    expect(setup.variation).toBe('partnership');
    await dealWithBots(client as never, pinochle, setup);
    expect(calls).toEqual(['bot', 'bot', 'bot', 'start']);
  });
});
