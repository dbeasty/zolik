import type { MatchState, Seat } from '@/src/api/matchTypes';
import { clock, tableBanner } from '@/src/components/match/TableBanner';

const NOW = Date.parse('2026-10-08T12:00:00Z');

function table(over: Partial<MatchState> = {}): MatchState {
  return {
    type: 'match_state',
    matchId: 'm1',
    moduleId: 'prsi',
    status: 'active',
    hostId: 'ada',
    players: [
      { id: 'ada', name: 'Ada', isAI: false },
      { id: 'bo', name: 'Bo', isAI: false },
    ],
    view: { zones: [] },
    ...over,
  } as unknown as MatchState;
}

const boOnTurn: Seat[] = [
  { playerId: 'ada', active: false } as Seat,
  { playerId: 'bo', active: true } as Seat,
];

const base = { seats: boOnTurn, viewerId: 'ada', connected: true, notice: null, now: NOW };

describe('tableBanner', () => {
  it('says you are offline before naming a missing player', () => {
    const b = tableBanner({ ...base, state: table({ status: 'suspended', suspendedPlayer: 'bo' }), connected: false });
    expect(b?.kind).toBe('offline');
  });

  it('says the server is offline before naming a missing player', () => {
    const b = tableBanner({ ...base, state: table({ status: 'suspended', suspendedPlayer: 'bo' }), serverGone: { name: "Ada's phone" } });
    expect(b?.kind).toBe('server');
    expect(b?.text).toContain("Ada's phone");
  });

  it('blames the server, not this player, when the server going took the socket down', () => {
    const b = tableBanner({ ...base, state: table(), connected: false, serverGone: { name: "Ada's phone" } });
    expect(b?.kind).toBe('server');
  });

  it('names who the table is waiting for, with the countdown to the bot', () => {
    const players = [
      { id: 'ada', name: 'Ada', isAI: false },
      { id: 'bo', name: 'Bo', isAI: false, standInAt: new Date(NOW + 42_000).toISOString() },
    ];
    const b = tableBanner({ ...base, state: table({ status: 'suspended', suspendedPlayer: 'bo', players } as Partial<MatchState>) });
    expect(b?.kind).toBe('waiting');
    expect(b?.playerId).toBe('bo');
    expect(b?.text).toContain('Bo');
    expect(b?.text).toContain('0:42');
  });

  it('counts down on an active table waiting on an away seat, too', () => {
    const players = [
      { id: 'ada', name: 'Ada', isAI: false },
      { id: 'bo', name: 'Bo', isAI: false, standInAt: new Date(NOW - 1000).toISOString() },
    ];
    const b = tableBanner({ ...base, state: table({ players } as Partial<MatchState>) });
    expect(b?.kind).toBe('waiting');
    expect(b?.text).not.toContain('0:');
  });

  it('does not tell a player the table is waiting for them', () => {
    const b = tableBanner({ ...base, viewerId: 'bo', state: table({ status: 'suspended', suspendedPlayer: 'bo' }) });
    expect(b).toBeNull();
  });

  it('welcomes a player back to a seat a bot was playing', () => {
    const b = tableBanner({ ...base, notice: 'stand_in_ended', state: table() });
    expect(b?.kind).toBe('back');
  });

  it('says who a bot is playing for, to everybody else', () => {
    const players = [
      { id: 'ada', name: 'Ada', isAI: false },
      { id: 'bo', name: 'Bo', isAI: false, standIn: { skill: 'medium', since: '', by: 'timeout' } },
    ];
    const b = tableBanner({ ...base, state: table({ players } as Partial<MatchState>) });
    expect(b?.kind).toBe('standin');
    expect(b?.text).toContain('Bo');
  });

  it('says nothing on a table that is simply playing', () => {
    expect(tableBanner({ ...base, state: table() })).toBeNull();
  });
});

describe('clock', () => {
  it('reads minutes and seconds', () => {
    expect(clock(0)).toBe('0:00');
    expect(clock(42)).toBe('0:42');
    expect(clock(125)).toBe('2:05');
  });
});
