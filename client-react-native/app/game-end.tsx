import { router } from 'expo-router';
import { Pressable, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { colors, shared } from '@/src/theme';

export default function GameEndScreen() {
  const { gameEnd, setGameEnd } = useGameFlow();

  if (!gameEnd) {
    return (
      <Screen title="Game complete">
        <Pressable onPress={() => router.replace('/')}>
          <Text style={shared.status}>Back to menu</Text>
        </Pressable>
      </Screen>
    );
  }

  const { state } = gameEnd;

  return (
    <Screen title="Game complete" scroll>
      {state.isDraw ? (
        <Text style={{ color: colors.muted, marginBottom: 12 }}>Draw game</Text>
      ) : null}
      <Text style={shared.status}>Final scores</Text>
      {state.players.map((p) => (
        <Text key={p.id} style={{ color: colors.text, marginBottom: 4 }}>
          {p.name}: {state.totalScores[p.id] ?? 0}
        </Text>
      ))}
      <Pressable
        style={[shared.button, { marginTop: 24 }]}
        onPress={() => {
          setGameEnd(null);
          router.replace('/');
        }}
      >
        <Text style={shared.buttonText}>Back to menu</Text>
      </Pressable>
    </Screen>
  );
}
