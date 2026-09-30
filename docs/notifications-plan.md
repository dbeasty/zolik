# Game notifications: nearby, circle and push

This is the design and wire contract for invites that reach a player without them going to look for a game:

- **Nearby tables.** A one-tap **Join** banner appears on every running app within Wi-Fi or Bluetooth range.
- **Circle notifications.** When a host opens a table for friends, the people in their **Game circle** are notified. If the app is open this is an in-app banner; if not, it is an OS push (Expo on iOS/Android, Web Push in browsers).

## Who gets notified

- **Nearby.** When an app hosts an offline table, it announces the table on Wi-Fi (Bonjour/NSD) and Bluetooth. Every app that is open nearby shows a banner. Nobody is seated until they tap Join. The app has no background modes, so this only works while the app is open. Browsers cannot discover local devices, so the web client gets online invites only.
- **Internet.** Invites go to the host's *circle*. Players are never notified just because they once shared a table. Past opponents appear as one-tap suggestions and must be added deliberately. There are three ways to add someone:
  - **Played before** (they appear in `match_results` alongside you): added immediately.
  - **Friend link or QR** (`/add/<code>`): added immediately, in both directions. Holding the link counts as consent.
  - **Username:** creates a *request* the other person has to accept.

Recipients stay in control:
- Invites can be switched to "off" for everyone.
- Any individual notifier can be muted.
- A per-device switch controls nearby banners.

Subject keys are `user:<hex>` and `guest:<id>`, the same format as `stats.MatchResult.SubjectKeys`, so guests take part too. When a guest signs in, their key is re-attributed together with their history.

## REST (every route needs a login unless marked public)

| Route | Body | Answer |
|---|---|---|
| `GET /notify/me` | | `Profile` |
| `PATCH /notify/me` | `{invites?: 'circle'\|'off', nearby?: boolean}` | `Profile` |
| `GET /notify/circle` | | `{members: Entry[], requests: Entry[], notifiers: Entry[]}` |
| `GET /notify/circle/suggestions` | | `{players: Suggestion[]}` |
| `POST /notify/circle` | `{key}` or `{username}` | `{entry: Entry}` |
| `DELETE /notify/circle/{key}` | | `204`. Stop notifying `key`. |
| `POST /notify/circle/{key}/mute` | `{muted: boolean}` | `204`. `key`'s invites to me are silenced. |
| `POST /notify/circle/requests/{key}/accept` | | `{entry: Entry}`. Both directions become active. |
| `POST /notify/circle/requests/{key}/decline` | | `204` |
| `GET /notify/friend/{code}` (public) | | `{name, avatar?}` |
| `POST /notify/friend/{code}` | | `{entry: Entry}`. Active in both directions. |
| `POST /notify/announce` | `{matchId, keys?: string[]}` | `{notified: number, already?: boolean}` |
| `GET /notify/config` (public) | | `{vapidPublicKey: string \| null, expo: boolean}` |
| `POST /notify/devices` | `{kind: 'expo'\|'webpush', token?, subscription?, platform, locale}` | `{id}` |
| `DELETE /notify/devices/{id}` | | `204` |

```ts
type Profile = { key: string; friendCode: string; friendUrl?: string; invites: 'circle' | 'off'; nearby: boolean };
type Entry = {
  key: string; name: string; avatar?: string;
  status: 'active' | 'pending';   // pending: waiting for the other side to accept
  since: string;                   // ISO time
  muted?: boolean;                 // notifiers only: I have muted them
};
type Suggestion = { key: string; name: string; avatar?: string; lastPlayedAt: string; matches: number };
```

Refusals use the usual `{code, message}` body. The codes are `NOT_PLAYED_TOGETHER`, `UNKNOWN_PLAYER`, `UNKNOWN_FRIEND_CODE`, `CANNOT_ADD_SELF`, `NOT_THE_HOST`, `MATCH_ALREADY_STARTED`, `RATE_LIMITED` and `PUSH_UNAVAILABLE`, rendered from `err.<CODE>`.

**Announce rules:**
- Only the host may announce, and only while the table is in lobby.
- Each circle member is told about a table at most once; a repeat call only reaches keys not told yet.
- A host may announce at most 6 tables an hour.
- A recipient is told about at most one table per host every 10 minutes.

## Socket: `GET /ws/me?token=`

This is one socket per signed-in client, opened for the whole app, not per screen. Server → client messages:

```ts
type Invite = {
  id: string;            // = matchId; one invite per table
  matchId: string; joinCode: string;
  moduleId: string; moduleLabel?: string; variation?: string;
  host: { key: string; name: string; avatar?: string };
  sentAt: string;
};
{ type: 'table_invite', invite: Invite }
{ type: 'invite_revoked', id: string }            // started, filled or deleted
{ type: 'lobby_invited', matchId, joinCode }      // seated from the waiting room (already joined)
{ type: 'circle_changed' }                        // a request arrived or was accepted
```

## Push payloads

The text is localised on the server, chosen by `device.locale` from `server/internal/notify/texts.json`, a file generated from the client bundles by `scripts/gen-notify-texts.js`.

- **Web push** (JSON body): `{type, invite?, id?, title, body, url, tag}`, where `tag` is `invite:<matchId>`. Revocations are **not** pushed to browsers: a web push must show a notification, and Chrome replaces a silent one with a generic "updated in the background" notice. The socket still revokes open tabs, and a stale invite's Join lands on the join screen's existing "already started" answer.
- **Expo:** `{to, title, body, data: {type, invite, url}, channelId: 'invites'}`. A revocation is data-only (`_contentAvailable`), because Expo cannot retract a notification already shown.

The client treats a push that arrives while it is in the foreground exactly like the socket message. Both carry the same `id`, so the two are de-duplicated.

## Known limits

- **Announcements are remembered in memory.** "Told once", the rate limits and revocation are all per server instance. A restart costs at most one repeated invite and one missed revocation.
- **Nearby banners show the host's name only.** The Bonjour TXT record and the Bluetooth info were left unchanged. Wi-Fi and Bluetooth sightings are de-duplicated by host name.
- **Native push needs an EAS `projectId`** in `app.config.ts` (`extra.eas.projectId`) and an EAS dev build. Until then the settings card reports that push is not set up.
