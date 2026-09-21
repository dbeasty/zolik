// Config plugin for the zolik-nearby module: everything the native projects
// need to host an offline table.
//
// Android: the embedded server arrives as an AAR in a flat Maven repository
// inside this module (see scripts/build-mobile-core.sh). The Android Gradle
// plugin will not accept a bare local .aar in a library module. The app
// project resolves the module's dependencies, so the app has to know the
// repository too, which is what this adds.
const { withProjectBuildGradle } = require('expo/config-plugins');

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

module.exports = function withZolikNearby(config) {
  return withZolikcoreRepository(config);
};
