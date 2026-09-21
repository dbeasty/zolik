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

  # Only this directory's own Swift. A `**` glob also reaches into the
  # framework above, and its C headers then end up in this pod's umbrella.
  s.source_files = "*.swift"
  s.pod_target_xcconfig = {
    'DEFINES_MODULE' => 'YES',
    'SWIFT_COMPILATION_MODE' => 'wholemodule'
  }
end
