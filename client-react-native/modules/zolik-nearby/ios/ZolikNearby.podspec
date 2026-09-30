require 'json'

package = JSON.parse(File.read(File.join(__dir__, '..', 'package.json')))

Pod::Spec.new do |s|
  s.name           = 'ZolikNearby'
  s.version        = package['version']
  s.summary        = package['description']
  s.description    = package['description']
  s.license        = package['license']
  s.author         = package['author']
  s.homepage       = package['homepage']
  s.platforms      = { :ios => '16.4' }
  s.swift_version  = '5.9'
  s.source         = { git: 'https://github.com/dbeasty/zolik.git' }
  s.static_framework = true

  s.dependency 'ExpoModulesCore'

  # Built by scripts/build-mobile-core.sh from server/mobile/zolikcore; not
  # checked in. `pod install` fails loudly without it, which is the point.
  s.vendored_frameworks = 'Zolikcore.xcframework'
  s.preserve_paths = 'Zolikcore.xcframework'

  # The Go runtime resolves host names through libresolv (res_9_ninit and
  # friends), so anything in the embedded server that dials a name rather than
  # an address needs it linked. Nothing did while the host only ever listened
  # on loopback and the local network; syncing with the cloud and fetching its
  # signing keys both do, and without this the app fails to link with three
  # undefined symbols that name neither.
  s.libraries = 'resolv'

  # Only this directory's own Swift. A `**` glob also reaches into the
  # framework above, and its C headers then end up in this pod's umbrella.
  s.source_files = "*.swift"
  s.pod_target_xcconfig = {
    'DEFINES_MODULE' => 'YES',
    'SWIFT_COMPILATION_MODE' => 'wholemodule'
  }
end
