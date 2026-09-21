// Config plugin for the zolik-nearby module: everything the native projects
// need to host an offline table.
//
// Android: the embedded server arrives as an AAR in a flat Maven repository
// inside this module (see scripts/build-mobile-core.sh). The Android Gradle
// plugin will not accept a bare local .aar in a library module. The app
// project resolves the module's dependencies, so the app has to know the
// repository too, which is what this adds.
const { AndroidConfig, withAndroidManifest, withInfoPlist, withProjectBuildGradle } = require('expo/config-plugins');

const REPO = 'maven { url "$rootDir/../modules/zolik-nearby/android/maven" }';

function withZolikcoreRepository(config) {
  return withProjectBuildGradle(config, (cfg) => {
    const gradle = cfg.modResults;
    if (gradle.language !== 'groovy') {
      throw new Error('zolik-nearby: expected a Groovy root build.gradle');
    }
    if (!gradle.contents.includes(REPO)) {
      const next = gradle.contents.replace(
        /allprojects\s*\{\s*repositories\s*\{/,
        (m) => `${m}\n    ${REPO}`,
      );
      if (next === gradle.contents) {
        throw new Error('zolik-nearby: could not find allprojects.repositories in build.gradle');
      }
      gradle.contents = next;
    }
    return cfg;
  });
}

// iOS: a table is found over Bonjour and joined over the local network.
// Both need declaring. iOS asks the player the first time either happens,
// using this sentence, and it silently finds nothing for a service type
// that is not listed.
function withLocalNetwork(config) {
  return withInfoPlist(config, (cfg) => {
    cfg.modResults.NSLocalNetworkUsageDescription =
      cfg.modResults.NSLocalNetworkUsageDescription ??
      'Zolik looks for card tables on this network so you can play with the people around you, with no internet needed.';
    const services = new Set(cfg.modResults.NSBonjourServices ?? []);
    services.add('_zolik._tcp');
    cfg.modResults.NSBonjourServices = [...services];
    return cfg;
  });
}

// Android: a table on the local network is plain http, to an address that
// is only known at run time. A network security config cannot name "any
// private address", so cleartext has to be allowed for the app as a whole.
// The online API is https regardless. Release builds need this said
// explicitly: `android.usesCleartextTraffic` in app.json only ever reached
// the debug manifest, and without this every join fails with "CLEARTEXT
// communication not permitted".
function withCleartextToTheRoom(config) {
  return withAndroidManifest(config, (cfg) => {
    const app = AndroidConfig.Manifest.getMainApplicationOrThrow(cfg.modResults);
    app.$['android:usesCleartextTraffic'] = 'true';
    return cfg;
  });
}

module.exports = function withZolikNearby(config) {
  return withCleartextToTheRoom(withLocalNetwork(withZolikcoreRepository(config)));
};
