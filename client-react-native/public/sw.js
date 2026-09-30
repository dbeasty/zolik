/*
 * The web client's service worker: Web Push and nothing else.
 *
 * It caches nothing and intercepts no requests. The site is served fresh by
 * the game server, and a worker that answered fetches from a cache would be a
 * second deployment to keep in step with the first.
 *
 * Payloads are `{type, invite?, id?, title, body, url, tag}` with `tag` =
 * `invite:<matchId>` (docs/notifications-plan.md). The text is already in the
 * player's language: the server picked it by the locale the device
 * registered with.
 */

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

function windows() {
  return self.clients.matchAll({ type: 'window', includeUncontrolled: true });
}

function tell(client, note) {
  client.postMessage(Object.assign({ source: 'zolik-sw' }, note));
}

self.addEventListener('push', (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch (e) {
    payload = {};
  }
  const tag = payload.tag || (payload.id ? 'invite:' + payload.id : undefined);

  event.waitUntil(
    (async () => {
      const open = await windows();

      // A table that started, filled or was deleted: take its notification
      // down, and let any open tab drop it from the banner too.
      if (payload.type === 'invite_revoked') {
        if (tag) {
          const shown = await self.registration.getNotifications({ tag });
          shown.forEach((n) => n.close());
        }
        open.forEach((c) => tell(c, { kind: 'push', payload }));
        return;
      }

      // Somebody is looking at the site right now: the page shows its own
      // banner, which is quieter and has the Join button in it.
      const looking = open.find((c) => c.focused || c.visibilityState === 'visible');
      if (looking) {
        tell(looking, { kind: 'push', payload });
        return;
      }

      await self.registration.showNotification(payload.title || 'Jokerless', {
        body: payload.body || '',
        tag,
        icon: '/icon-192.png',
        badge: '/icon-192.png',
        data: { url: payload.url || '/', payload },
      });
    })(),
  );
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const data = event.notification.data || {};
  const url = new URL(data.url || '/', self.location.origin).href;

  event.waitUntil(
    (async () => {
      const open = await windows();
      // An open tab of this site is reused rather than a second one opened:
      // it already has the session, and it hands the route to the router
      // instead of reloading the app.
      const same = open.find((c) => new URL(c.url).origin === self.location.origin);
      if (same) {
        await same.focus();
        tell(same, { kind: 'open', url, payload: data.payload });
        return;
      }
      await self.clients.openWindow(url);
    })(),
  );
});
