import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import type { ConfigContext, ExpoConfig } from 'expo/config';

/**
 * The native apps' identity, layered over app.json.
 *
 * app.json stays the static base (the web build, docs and the Dockerfile all
 * describe it); what lives here is what has to be computed or only matters
 * to the iOS and Android binaries.
 *
 * The identifier is permanent once a store record exists — App Store Connect
 * and Play both refuse to rename one — so it is spelled once, here.
 */
const APP_ID = 'com.jokerless.app';

/** The repo's VERSION file, the one source every other build reads. */
function releaseVersion(): string {
  try {
    const v = readFileSync(join(__dirname, '..', 'VERSION'), 'utf8').split('\n')[0].trim();
    if (/^\d+\.\d+\.\d+$/.test(v)) return v;
  } catch {
    // Fall through: a tarball without the file is not a release.
  }
  return '0.0.0';
}

/**
 * The commit this binary was built from. An EAS worker builds from an
 * archive with no .git, but tells us the hash in its environment; a local
 * build asks git.
 */
function buildCommit(): string {
  const fromEas = process.env.EAS_BUILD_GIT_COMMIT_HASH;
  if (fromEas) return fromEas.slice(0, 7);
  try {
    return execSync('git rev-parse --short=7 HEAD', { stdio: ['ignore', 'pipe', 'ignore'] })
      .toString()
      .trim();
  } catch {
    return 'unknown';
  }
}

export default ({ config }: ConfigContext): ExpoConfig => {
  const version = releaseVersion();
  const production = process.env.EAS_BUILD_PROFILE === 'production';

  return {
    ...config,
    // Matches app.json's own name, so the web build, the in-app menu (see
    // app/index.tsx) and the iOS/Android home-screen label all say the same
    // thing. `slug` stays 'zolik' regardless — it is the EAS project's own
    // identifier, not something a player ever reads.
    name: 'Jokerless',
    slug: 'zolik',
    version,
    ios: {
      ...config.ios,
      bundleIdentifier: APP_ID,
      infoPlist: {
        ...config.ios?.infoPlist,
        // Arbitrary loads are for pointing a dev build at a LAN server over
        // plain http; a store build only ever talks https to the public API.
        // Local networking stays on for both: the nearby-play host is local.
        NSAppTransportSecurity: {
          NSAllowsArbitraryLoads: !production,
          NSAllowsLocalNetworking: true,
        },
        ITSAppUsesNonExemptEncryption: false,
      },
    },
    android: {
      ...config.android,
      package: APP_ID,
      // The prebuild template asks for these; nothing in the app uses them,
      // and Play review asks why an app wants to draw over other apps.
      blockedPermissions: [
        'android.permission.SYSTEM_ALERT_WINDOW',
        'android.permission.READ_EXTERNAL_STORAGE',
        'android.permission.WRITE_EXTERNAL_STORAGE',
      ],
    },
    plugins: [
      ...(config.plugins ?? []),
      'expo-dev-client',
      './modules/zolik-nearby/plugin/withZolikNearby.js',
      // Table invites from the player's circle. `invites` is the channel the
      // server names in every push (see src/notify/push.native.ts), made the
      // default so a push that names none still lands somewhere sensible.
      ['expo-notifications', { defaultChannel: 'invites', color: '#1a2332' }],
    ],
    // Read by src/config.ts when the bundler was not given
    // EXPO_PUBLIC_ZOLIK_VERSION — i.e. on an EAS worker, which runs neither
    // scripts/version.sh nor git.
    extra: {
      ...config.extra,
      zolikVersion: version,
      zolikCommit: buildCommit(),
    },
  };
};
