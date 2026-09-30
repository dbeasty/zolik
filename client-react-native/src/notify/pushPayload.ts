import type { MeWSMessage, TableInvite } from '@/src/api/types';

/**
 * A push's data, read back into the socket's vocabulary.
 *
 * Web push sends `{type, invite?, id?, title, body, url, tag}` and Expo sends
 * `data: {type, invite, url}` (docs/notifications-plan.md). Both are narrowed
 * to a `MeWSMessage` here, so a push that arrives with the app open goes down
 * exactly the path the socket's copy does — same id, so the two de-duplicate.
 * Anything malformed is dropped: the payload came over the network.
 */
export function messageFromPush(data: unknown): MeWSMessage | null {
  if (!data || typeof data !== 'object') return null;
  const d = data as Record<string, unknown>;
  switch (d.type) {
    case 'table_invite': {
      const invite = d.invite as TableInvite | undefined;
      if (!invite || typeof invite !== 'object' || !invite.matchId) return null;
      return { type: 'table_invite', invite: { ...invite, id: invite.id || invite.matchId } };
    }
    case 'invite_revoked': {
      const id = typeof d.id === 'string' ? d.id : tagId(d.tag);
      return id ? { type: 'invite_revoked', id } : null;
    }
    case 'lobby_invited':
      return typeof d.matchId === 'string'
        ? { type: 'lobby_invited', matchId: d.matchId, joinCode: String(d.joinCode ?? '') }
        : null;
    case 'circle_changed':
      return { type: 'circle_changed' };
    default:
      return null;
  }
}

/** `invite:<matchId>` → `<matchId>`. */
function tagId(tag: unknown): string {
  return typeof tag === 'string' && tag.startsWith('invite:') ? tag.slice('invite:'.length) : '';
}
