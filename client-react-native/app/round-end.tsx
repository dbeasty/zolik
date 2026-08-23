import { router } from 'expo-router';
import { Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { colors, shared } from '@/src/theme';

export default function RoundEndScreen() {
  const { roundEnd, setRoundEnd } = useGameFlow();

  if (!roundEnd) {
    return (
      <Screen title="Deal complete">
        <Pressable onPress={() => router.replace('/')}>
          <Text style={shared.status}>Back to menu</Text>
        </Pressable>
      </Screen>
    );
  }

  const { state, data, gameId } = roundEnd;
  const winnerId = String(data.winnerId ?? '');
  const winner = state.players.find((p) => p.id === winnerId);

  return (
    <Screen title={`Game ${state.game}: deal complete`} scroll>
      {winner ? (
        <Text style={{ color: colors.text, fontSize: 16, marginBottom: 12 }}>
          {winner.name} went out!
        </Text>
      ) : null}
      <Text style={shared.status}>Running totals</Text>
      {state.players.map((p) => (
        <Text key={p.id} style={{ color: colors.text, marginBottom: 4 }}>
          {p.name}: {state.totalScores[p.id] ?? 0}
        </Text>
      ))}
      <Pressable
        style={[shared.button, { marginTop: 24 }]}
        onPress={() => {
          setRoundEnd(null);
          // *Return* to the game screen rather than mounting a new one. This
          // screen was pushed on top of the game (see onRoundEnd in
          // app/game/[gameId].tsx), so that screen is still mounted below,
          // still holding its WebSocket. A `replace` here swaps out only this
          // entry and leaves a second, stacked game screen mounted — and since
          // the server permits one connection per player per game and evicts
          // the older one (server/internal/game/handler.go), the two screens
          // then evict each other and auto-reconnect forever, leaving the
          // visible one stuck on "reconnecting…". dismissTo pops back to the
          // existing screen, falling back to a replace if there isn't one.
          router.dismissTo(`/game/${gameId}`);
        }}
      >
        <Text style={shared.buttonText}>Continue</Text>
      </Pressable>
    </Screen>
  );
}
