/**
 * One invitation to a table, whichever way it arrived.
 *
 * Four roads lead here — a phone in the room on Wi-Fi, one over Bluetooth, a
 * circle member's online table, and the waiting room seating this player —
 * and the banner, the pill and the home screen's list all read this one
 * shape, so none of them has to know which road an invite took.
 */

export type InviteSource = 'wifi' | 'ble' | 'online' | 'waiting-room';

/** Where tapping Join goes. */
export type InviteTarget =
  | {
      kind: 'online';
      matchId: string;
      joinCode: string;
      /**
       * Already seated: the waiting room put this player at the table before
       * telling them, so Join only has to take them there. Everything else
       * online still needs the seat taken.
       */
      seated?: boolean;
    }
  | {
      kind: 'nearby';
      /** The host's instance id, known over Wi-Fi only. */
      instanceId?: string;
      /** `ip:port`, over Wi-Fi. */
      address?: string;
      /** Over Bluetooth. */
      peripheralId?: string;
    };

export type Invite = {
  /**
   * What de-duplicates. The match id online, so the socket's copy and the
   * push's are one invite; `nearby:<host name>` in the room, so the same
   * table seen over Wi-Fi and over Bluetooth is one invite too.
   */
  id: string;
  source: InviteSource;
  host: {
    name: string;
    avatar?: string;
    /** Online invites only: whom "mute" silences. */
    key?: string;
    /** This phone has sat at the host's table before. */
    known?: boolean;
  };
  moduleId?: string;
  moduleLabel?: string;
  variation?: string;
  target: InviteTarget;
  /** When it first arrived, in ms. Later sightings leave this alone. */
  receivedAt: number;
  /** When it was last heard of, in ms — nearby tables go quiet rather than
   *  saying goodbye, so this is how one that left is noticed. */
  seenAt: number;
  /**
   * The banner has had its turn: it timed out, or the player looked at it in
   * the list. It stays queued for the home screen and the pill.
   */
  shelved?: boolean;
};
