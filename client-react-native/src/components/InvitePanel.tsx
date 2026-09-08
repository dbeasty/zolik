import { useEffect, useState } from 'react';
import { Platform, Pressable, Text, View } from 'react-native';

import { inviteUrlFor, shareInviteLink } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';

/**
 * The host's "invite people" control: one link, one button.
 *
 * This is the screen half of the Zoom-shaped flow. What made the old table
 * screen unlike Zoom was not that it lacked a code — it had one, in bold —
 * but that the code was only usable by somebody already looking at a second
 * screen with the app open on the right page. A link is usable by somebody
 * looking at their phone in a different room, which is the whole difference.
 *
 * So the link is the loud thing and the code is the fallback underneath it,
 * for reading down a phone line or typing on a device that cannot follow
 * links. Both are always shown: neither one is right for every situation, and
 * hiding one behind a disclosure would mean guessing which.
 *
 * The URL is rendered as selectable text as well as being copyable, because
 * clipboard writes are refused more often than one would like — insecure
 * origins, a browser wanting a fresher gesture — and a host who cannot copy
 * must still be able to select the thing by hand.
 */
export function InvitePanel({
  joinCode,
  inviteUrl,
}: {
  joinCode?: string;
  inviteUrl?: string;
}) {
  const url = inviteUrlFor({ joinCode, inviteUrl });
  const [done, setDone] = useState(false);

  // The confirmation is a moment, not a state. Left up permanently it stops
  // meaning "that worked just now", which is the only thing it is for.
  useEffect(() => {
    if (!done) return;
    const t = setTimeout(() => setDone(false), 2000);
    return () => clearTimeout(t);
  }, [done]);

  // On a phone the button opens the system share sheet, which is where the
  // recipient actually is; on web it copies, because that is what sharing a
  // URL means in a browser.
  const actionLabel = Platform.OS === 'web' ? 'Copy link' : 'Share link';
  const doneLabel = Platform.OS === 'web' ? 'Copied!' : 'Shared';

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="invite-panel">
      <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14 }}>
        Invite players
      </Text>
      <Text style={shared.status}>
        Send this link. Whoever opens it lands at this table — no account needed.
      </Text>

      {url ? (
        <>
          {/*
            Selectable, and wrapping rather than truncated. A host reading the
            link back to check it got the right table is doing something
            reasonable, and an ellipsis in the middle of a URL defeats both
            that and a manual selection.
          */}
          <Text
            testID="invite-url"
            selectable
            style={{
              color: colors.accent,
              marginTop: 10,
              fontSize: 14,
              // A URL is a string of characters that has to be read one at a
              // time to be checked; a proportional face makes that harder for
              // exactly the reason it makes prose easier.
              fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace',
            }}
          >
            {url}
          </Text>
          <Pressable
            testID="invite-share"
            style={[shared.button, { marginTop: 12, marginBottom: 0 }]}
            onPress={async () => {
              setDone(await shareInviteLink(url, 'Join my table on Žolíky'));
            }}
          >
            <Text style={shared.buttonText}>{done ? doneLabel : actionLabel}</Text>
          </Pressable>
        </>
      ) : (
        // No link to offer: an older server, or one with no public address
        // configured. Said plainly rather than shown as a dead button — the
        // code below still works, and that is the useful thing to point at.
        <Text testID="invite-url-unavailable" style={shared.status}>
          This server has no shareable address configured, so use the code below.
        </Text>
      )}

      {joinCode ? (
        <Text style={{ color: colors.muted, fontSize: 13, marginTop: 12 }}>
          Or read out the code:{' '}
          <Text testID="table-join-code" style={{ color: colors.text, fontWeight: '700', fontSize: 16 }}>
            {joinCode}
          </Text>
        </Text>
      ) : null}
    </View>
  );
}
