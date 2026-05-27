import { router } from 'expo-router';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';

function MenuButton({
  label,
  onPress,
  secondary,
}: {
  label: string;
  onPress: () => void;
  secondary?: boolean;
}) {
  return (
    <Pressable
      style={[shared.button, secondary && shared.buttonSecondary]}
      onPress={onPress}
    >
      <Text style={[shared.buttonText, secondary && shared.buttonTextSecondary]}>
        {label}
      </Text>
    </Pressable>
  );
}

export default function MainMenu() {
  const { session, loading, logout } = useSession();

  if (loading) {
    return (
      <Screen>
        <ActivityIndicator color="#3d8bfd" />
      </Screen>
    );
  }

  return (
    <Screen
      title="Žolíky"
      subtitle={`Continental Rummy · ${ZOLIK_BASE_URL}`}
      scroll
    >
      {session ? (
        <Text style={shared.status}>Playing as {session.username}</Text>
      ) : (
        <Text style={shared.status}>Sign in or continue as guest to play online.</Text>
      )}

      <View style={{ marginTop: 16 }}>
        <MenuButton
          label="New game"
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/create');
          }}
        />
        <MenuButton
          label="Join game"
          secondary
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/join');
          }}
        />
        <MenuButton
          label="Offline score table"
          secondary
          onPress={() => router.push('/scoring')}
        />
        <MenuButton label="Stats & leaderboard" secondary onPress={() => router.push('/stats')} />

        {session ? (
          <>
            <MenuButton label="Sign out" secondary onPress={() => logout()} />
          </>
        ) : (
          <>
            <MenuButton label="Continue as guest" onPress={() => router.push('/auth/guest')} />
            <MenuButton label="Sign in" secondary onPress={() => router.push('/auth/login')} />
            <MenuButton
              label="Create account"
              secondary
              onPress={() => router.push('/auth/register')}
            />
          </>
        )}
      </View>
    </Screen>
  );
}
