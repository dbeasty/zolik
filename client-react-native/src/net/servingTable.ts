import { Platform } from 'react-native';

import { ZOLIK_BASE_URL } from '@/src/config';

/**
 * Whether this page was served by a phone hosting a table, rather than by the
 * cloud.
 *
 * A phone in the room serves the same web client the cloud does, so that a
 * person with no app can open the link the host shared and play in their
 * browser. The bundle cannot tell the two apart by itself; the server can.
 * Only a phone answers `/nearby/info`, with the id of the table it hosts.
 *
 * Asked only on web, and only of the origin the page came from: a native app
 * knows whether it is at a table because it went and sat at one.
 */
export async function servingTable(): Promise<{ instanceId: string } | null> {
  if (Platform.OS !== 'web') return null;
  if (process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN !== '1') return null;
  try {
    const res = await fetch(`${ZOLIK_BASE_URL}/nearby/info`, { headers: { Accept: 'application/json' } });
    if (!res.ok) return null;
    const info = (await res.json()) as { instanceId?: unknown };
    return typeof info.instanceId === 'string' && info.instanceId ? { instanceId: info.instanceId } : null;
  } catch {
    return null;
  }
}
