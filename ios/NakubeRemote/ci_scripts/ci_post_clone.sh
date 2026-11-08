#!/usr/bin/env bash
# Validate source identity, without overriding Xcode Cloud's build numbering.
set -euo pipefail

root="${CI_PRIMARY_REPOSITORY_PATH:-$(cd "$(dirname "$0")/../../.." && pwd)}"
exec ruby - "$root/ios/NakubeRemote/NakubeRemote.xcodeproj/project.pbxproj" <<'RUBY'
source = File.read(ARGV.fetch(0))
versions = source.scan(/\bMARKETING_VERSION = ([^;]+);/).flatten
builds = source.scan(/\bCURRENT_PROJECT_VERSION = ([^;]+);/).flatten
abort "error: Debug/Release marketing versions must match" unless versions.length == 2 && versions.uniq.length == 1
abort "error: Debug/Release source build numbers must match" unless builds.length == 2 && builds.uniq.length == 1

tag = ENV.fetch("CI_TAG", "")
unless tag.empty?
  match = /\Aios\/v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))(?:-build\.([1-9]\d*))?\z/.match(tag)
  abort "error: expected an iOS tag (ios/v1.2.3 or ios/v1.2.3-build.2), got #{tag}" unless match
  abort "error: tag marketing version does not match project" unless match[1] == versions.first
  abort "error: tag source build number does not match project" if match[2] && match[2] != builds.first
end

puts "NaKube iOS #{versions.first}; source build #{builds.first}; tag #{tag.empty? ? '(branch/manual)' : tag}"
puts "Distributed build number is assigned by Xcode Cloud: #{ENV.fetch('CI_BUILD_NUMBER', '(not running in Xcode Cloud)')}"
RUBY
