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
    // Bluetooth, for tables with no network at all. The phones never pair,
    // so this is the only prompt a player sees for it.
    cfg.modResults.NSBluetoothAlwaysUsageDescription =
      cfg.modResults.NSBluetoothAlwaysUsageDescription ??
      'Zolik uses Bluetooth to play cards with the people around you when there is no Wi-Fi.';
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

// Android Bluetooth. From API 31 the three runtime permissions below are all
// a table needs, and "neverForLocation" says scanning is not used to locate
// anybody, so no location permission is asked. Up to API 30 scanning needs
// the legacy pair plus fine location, capped so newer phones never see them.
// Bluetooth LE is optional hardware: a phone without it still plays online
// and over Wi-Fi.
const BLE_PERMISSIONS = [
  ['android.permission.BLUETOOTH_SCAN', { 'android:usesPermissionFlags': 'neverForLocation' }],
  ['android.permission.BLUETOOTH_CONNECT', {}],
  ['android.permission.BLUETOOTH_ADVERTISE', {}],
  ['android.permission.BLUETOOTH', { 'android:maxSdkVersion': '30' }],
  ['android.permission.BLUETOOTH_ADMIN', { 'android:maxSdkVersion': '30' }],
  ['android.permission.ACCESS_FINE_LOCATION', { 'android:maxSdkVersion': '30' }],
];

function withBluetoothPermissions(config) {
  return withAndroidManifest(config, (cfg) => {
    const manifest = cfg.modResults.manifest;
    manifest.$['xmlns:tools'] = manifest.$['xmlns:tools'] ?? 'http://schemas.android.com/tools';
    const perms = (manifest['uses-permission'] = manifest['uses-permission'] ?? []);
    for (const [name, attrs] of BLE_PERMISSIONS) {
      const existing = perms.find((p) => p.$['android:name'] === name);
      if (existing) Object.assign(existing.$, attrs);
      else perms.push({ $: { 'android:name': name, ...attrs } });
    }
    const features = (manifest['uses-feature'] = manifest['uses-feature'] ?? []);
    if (!features.some((f) => f.$['android:name'] === 'android.hardware.bluetooth_le')) {
      features.push({ $: { 'android:name': 'android.hardware.bluetooth_le', 'android:required': 'false' } });
    }
    return cfg;
  });
}

module.exports = function withZolikNearby(config) {
  return withBluetoothPermissions(
    withCleartextToTheRoom(withLocalNetwork(withZolikcoreRepository(config))),
  );
};
