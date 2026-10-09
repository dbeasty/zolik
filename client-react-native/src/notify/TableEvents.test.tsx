import React from 'react';
import { act, create } from 'react-test-renderer';

import { TableEventsProvider, useAnnounceArrivals, useTableEvents } from '@/src/notify/TableEvents';

jest.mock('expo-router', () => ({ router: { push: jest.fn() } }));
jest.mock('react-native-safe-area-context', () => ({ useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }) }));
jest.mock('@/src/components/avatars/Avatar', () => ({ Avatar: () => null }));

type P = { id: string; name: string; isAI: boolean };

function Probe({ players, viewer }: { players: P[]; viewer: string }) {
  useAnnounceArrivals('m1', players, viewer, { moduleId: 'prsi', joinCode: 'ABC123' });
  return null;
}

function Socket({ fire }: { fire: (announce: ReturnType<typeof useTableEvents>['announce']) => void }) {
  const { announce } = useTableEvents();
  React.useEffect(() => fire(announce), [announce, fire]);
  return null;
}

const ada: P = { id: 'ada', name: 'Ada', isAI: false };
const bea: P = { id: 'bea', name: 'Bea', isAI: false };
const bot: P = { id: 'bot', name: 'Bot', isAI: true };

function banners(tree: ReturnType<typeof create>) {
  return tree.root.findAll((n) => n.props.testID === 'arrival-banner' && typeof n.type !== 'string');
}

describe('who sat down', () => {
  it('announces nobody for the players already there, then each newcomer but bots and the viewer', () => {
    let tree!: ReturnType<typeof create>;
    act(() => {
      tree = create(
        <TableEventsProvider>
          <Probe players={[ada]} viewer="ada" />
        </TableEventsProvider>,
      );
    });
    expect(banners(tree)).toHaveLength(0);

    act(() =>
      tree.update(
        <TableEventsProvider>
          <Probe players={[ada, bot]} viewer="ada" />
        </TableEventsProvider>,
      ),
    );
    expect(banners(tree)).toHaveLength(0);

    act(() =>
      tree.update(
        <TableEventsProvider>
          <Probe players={[ada, bot, bea]} viewer="ada" />
        </TableEventsProvider>,
      ),
    );
    const shown = banners(tree);
    expect(shown).toHaveLength(1);
    expect(JSON.stringify(tree.toJSON())).toContain('Bea');
  });

  it('shows the same arrival once, whether the cloud or the player list said it first', () => {
    let tree!: ReturnType<typeof create>;
    const fire = (announce: ReturnType<typeof useTableEvents>['announce']) => {
      announce({ matchId: 'm1', playerId: 'bea', name: 'Bea' });
    };
    act(() => {
      tree = create(
        <TableEventsProvider>
          <Socket fire={fire} />
          <Probe players={[ada]} viewer="ada" />
        </TableEventsProvider>,
      );
    });
    expect(banners(tree)).toHaveLength(1);
    // The banner goes; the list then shows Bea too, and that is not news again.
    act(() => {
      tree.root.findAll((n) => n.props.testID === 'arrival-banner')[0]?.props.onPress?.();
    });
    act(() =>
      tree.update(
        <TableEventsProvider>
          <Socket fire={fire} />
          <Probe players={[ada, bea]} viewer="ada" />
        </TableEventsProvider>,
      ),
    );
    expect(banners(tree)).toHaveLength(0);
  });
});
