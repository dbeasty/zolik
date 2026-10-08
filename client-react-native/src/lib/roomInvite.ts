import * as nearby from '@/modules/zolik-nearby';
import type { OfflineTable } from '@/src/context/SessionContext';
import { INVITE_PATH, inviteUrlFor } from '@/src/lib/inviteLink';

/**
 * The link to a table on a phone, for somebody in the same room.
 *
 * The phone serves the web client as well as the table, so the link opens in
 * any browser on the same Wi-Fi or hotspot, with nothing to install. It names
 * the phone by its address on that network, which is why it only works there,
 * and only while the phone is hosting.
 *
 *  - On the phone hosting the table: its own Wi-Fi address and room port.
 *  - At that table in the app, over Wi-Fi: the address this device reached it by.
 *  - In a browser the phone served: the page's own address.
 *  - Over Bluetooth there is no address anybody else could use.
 */
export function roomInviteUrl(
  table: Pick<OfflineTable, 'role' | 'via' | 'baseUrl'>,
  joinCode: string | undefined,
  servedByTable: boolean,
  hostAddress: () => { address: string; port: number } | null = phoneAddress,
): string {
  const code = (joinCode ?? '').trim();
  if (!code) return '';
  if (servedByTable) return inviteUrlFor({ joinCode: code });
  let base = '';
  if (table.role === 'host') {
    const here = hostAddress();
    if (here) base = `http://${here.address}:${here.port}`;
  } else if (table.via === 'wifi' && /^https?:\/\//.test(table.baseUrl)) {
    base = table.baseUrl.replace(/\/$/, '');
  }
  return base ? base + INVITE_PATH + encodeURIComponent(code) : '';
}

/** This phone's address on the room's network, while it has the room open. */
export function phoneAddress(): { address: string; port: number } | null {
  const port = nearby.hostStatus()?.lanPort;
  const address = nearby.localAddresses()[0];
  return port && address ? { address, port } : null;
}
