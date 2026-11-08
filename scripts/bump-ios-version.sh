#!/usr/bin/env bash
# Update the iOS version and print release commands. Never commits, tags, or pushes.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
exec ruby - "$root" "$@" <<'RUBY'
require "optparse"
require "shellwords"

root = ARGV.shift
options = { dry_run: false, build_only: false }
parser = OptionParser.new do |opts|
  opts.banner = "Usage: scripts/bump-ios-version.sh [--dry-run] [--build-only | VERSION]"
  opts.separator ""
  opts.separator "Set iOS MARKETING_VERSION and increment CURRENT_PROJECT_VERSION."
  opts.separator "Without VERSION, increment only the source build number."
  opts.separator "Print explicit commit/tag/push commands; never execute them."
  opts.separator "Xcode Cloud assigns the distributed build number independently."
  opts.on("--dry-run", "Preview without changing files") { options[:dry_run] = true }
  opts.on("--build-only", "Keep marketing version and increment build") { options[:build_only] = true }
  opts.on("-h", "--help", "Show help") { puts opts; exit }
end

begin
  parser.parse!
  raise "Provide at most one version" if ARGV.length > 1
  raise "--build-only cannot be combined with VERSION" if options[:build_only] && !ARGV.empty?

  requested = ARGV.first&.delete_prefix("v")
  version_pattern = /\A(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\z/
  raise "Expected VERSION in major.minor.patch format (for example 1.2.3)" if requested && !version_pattern.match?(requested)

  relative = "ios/NakubeRemote/NakubeRemote.xcodeproj/project.pbxproj"
  path = File.join(root, relative)
  source = File.read(path)
  versions = source.scan(/\bMARKETING_VERSION = ([^;]+);/).flatten
  builds = source.scan(/\bCURRENT_PROJECT_VERSION = ([^;]+);/).flatten
  unless versions.length == 2 && versions.uniq.length == 1 && version_pattern.match?(versions.first)
    raise "Expected matching Debug/Release MARKETING_VERSION settings in #{relative}"
  end
  unless builds.length == 2 && builds.uniq.length == 1 && /\A[1-9]\d*\z/.match?(builds.first)
    raise "Expected matching positive integer Debug/Release CURRENT_PROJECT_VERSION settings in #{relative}"
  end

  current_version = versions.first
  version = requested || current_version
  if (version.split(".").map(&:to_i) <=> current_version.split(".").map(&:to_i)) == -1
    raise "Refusing to lower marketing version from #{current_version} to #{version}"
  end
  build = Integer(builds.first, 10) + 1
  tag = requested ? "ios/v#{version}" : "ios/v#{version}-build.#{build}"
  exists = system("git", "-C", root, "show-ref", "--verify", "--quiet", "refs/tags/#{tag}")
  raise "Tag #{tag} already exists; use --build-only for another build" if exists
  raise "Cannot inspect local tags" unless $?.exitstatus == 1

  updated = source.gsub(/\bMARKETING_VERSION = [^;]+;/, "MARKETING_VERSION = #{version};")
                  .gsub(/\bCURRENT_PROJECT_VERSION = [^;]+;/, "CURRENT_PROJECT_VERSION = #{build};")
  File.write(path, updated) unless options[:dry_run]

  puts "#{options[:dry_run] ? 'Would update' : 'Updated'} #{relative}"
  puts "  MARKETING_VERSION: #{current_version} -> #{version}"
  puts "  CURRENT_PROJECT_VERSION (source/local): #{builds.first} -> #{build}"
  puts "  Tag: #{tag}"
  puts ""
  puts "Review changes and run scripts/check-swift.sh before releasing."
  puts "After checking that remote tags are up to date, run from the repository root:"
  puts "  git add -- #{relative.shellescape}"
  puts "  git commit -m #{"chore(ios): release #{version} build #{build}".shellescape}"
  puts "  git tag -a #{tag.shellescape} -m #{"NaKube iOS #{version} source build #{build}".shellescape}"
  puts "  git push origin HEAD"
  puts "  git push origin #{"refs/tags/#{tag}".shellescape}"
  puts "Only push the explicit iOS tag; do not use --tags or --follow-tags."
  puts "Xcode Cloud uses MARKETING_VERSION plus its own CI_BUILD_NUMBER for TestFlight."
rescue OptionParser::ParseError, RuntimeError, SystemCallError => error
  warn "error: #{error.message}"
  exit 1
end
RUBY
