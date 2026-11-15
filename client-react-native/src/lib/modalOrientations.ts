import type { ModalProps } from 'react-native';

/**
 * Every way up a sheet may be drawn, for React Native's `Modal`.
 *
 * On iOS a `Modal` is its own view controller, and without this it allows
 * portrait only: opening one in landscape turned the whole phone back upright
 * (a bot's level, picked from its seat, did it every time). The app's own
 * rotation setting still applies — UIKit keeps the intersection with the lock
 * `rotationPref` set — so a phone held in portrait stays there.
 */
export const MODAL_ORIENTATIONS: NonNullable<ModalProps['supportedOrientations']> = [
  'portrait',
  'portrait-upside-down',
  'landscape',
  'landscape-left',
  'landscape-right',
];
