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
          router.replace(`/game/${gameId}`);
        }}
      >
        <Text style={shared.buttonText}>Continue</Text>
      </Pressable>
    </Screen>
  );
}
