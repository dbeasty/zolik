import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';

import { apiClient } from '@/src/api/client';
import { CLIENT_COMMIT, CLIENT_VERSION } from '@/src/config';
import { shared } from '@/src/theme';

type ServerBuild = { version: string; commit: string };

/**
 * The build this app is running, next to the build the server answered with
 * — so "is the fix in?" is answerable by looking at the screen instead of
 * reading logs. Fetched once on mount, not polled: unlike waiting-room status
 * this never changes without a full reload. A failed fetch (unreachable
 * server, cold start) just leaves the server line as a placeholder — this
 * must never surface as an error on the main menu.
 */
export function BuildFooter() {
  const [server, setServer] = useState<ServerBuild | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiClient
      .getVersion()
      .then((build) => {
        if (!cancelled) setServer(build);
      })
      .catch(() => {
        // Best-effort only — see the comment above.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <View testID="build-footer" style={{ marginTop: 24 }}>
      <Text style={shared.status} testID="build-footer-app">
        app {CLIENT_VERSION} · {CLIENT_COMMIT}
      </Text>
      <Text style={shared.status} testID="build-footer-server">
        {server ? `server ${server.version} · ${server.commit}` : 'server …'}
      </Text>
    </View>
  );
}
