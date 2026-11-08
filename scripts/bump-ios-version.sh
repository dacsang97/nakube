#!/usr/bin/env bash
# Bump the iOS version, commit, tag, and atomically push the release.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
exec ruby - "$root" "$@" <<'RUBY'
require "optparse"
require "shellwords"
require "open3"

root = ARGV.shift
options = { dry_run: false, build_only: false }
parser = OptionParser.new do |opts|
  opts.banner = "Usage: scripts/bump-ios-version.sh [--dry-run] [--build-only | VERSION]"
  opts.separator ""
  opts.separator "Set iOS MARKETING_VERSION and increment CURRENT_PROJECT_VERSION."
  opts.separator "Without VERSION, increment only the source build number."
  opts.separator "Requires clean main; commits, tags, and atomically pushes to origin."
  opts.separator "Xcode Cloud assigns the distributed build number independently."
  opts.on("--dry-run", "Check and preview without changing files, committing, tagging, or pushing") { options[:dry_run] = true }
  opts.on("--build-only", "Keep marketing version and increment build") { options[:build_only] = true }
  opts.on("-h", "--help", "Show help") { puts opts; exit }
end

git = lambda do |*args|
  stdout, stderr, status = Open3.capture3("git", "-C", root, *args)
  raise "git #{args.shelljoin} failed: #{stderr.strip}" unless status.success?
  stdout
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
  tag = requested ? "ios-v#{version}" : "ios-v#{version}-build.#{build}"
  raise "Release must run on main" unless git.call("branch", "--show-current").strip == "main"
  raise "Working tree must be clean; commit or stash changes first" unless git.call("status", "--porcelain", "--untracked-files=all").empty?
  git.call("remote", "get-url", "origin")
  git.call("var", "GIT_AUTHOR_IDENT")
  git.call("var", "GIT_COMMITTER_IDENT")
  git.call("fetch", "--quiet", "--tags", "origin", "refs/heads/main:refs/remotes/origin/main")
  behind = git.call("rev-list", "--count", "HEAD..refs/remotes/origin/main").strip
  raise "main is behind or diverged from origin/main; update it first" unless behind == "0"
  raise "Tag #{tag} already exists; use --build-only for another build" unless git.call("tag", "--list", tag).empty?

  updated = source.gsub(/\bMARKETING_VERSION = [^;]+;/, "MARKETING_VERSION = #{version};")
                  .gsub(/\bCURRENT_PROJECT_VERSION = [^;]+;/, "CURRENT_PROJECT_VERSION = #{build};")
  File.write(path, updated) unless options[:dry_run]

  puts "#{options[:dry_run] ? 'Would update' : 'Updated'} #{relative}"
  puts "  MARKETING_VERSION: #{current_version} -> #{version}"
  puts "  CURRENT_PROJECT_VERSION (source/local): #{builds.first} -> #{build}"
  puts "  Tag: #{tag}"
  puts ""
  push = ["-c", "push.followTags=false", "push", "--atomic", "origin",
          "HEAD:refs/heads/main", "refs/tags/#{tag}:refs/tags/#{tag}"]
  if options[:dry_run]
    puts "Would commit the version settings, create annotated tag #{tag}, and run:"
    puts "  git #{push.shelljoin}"
    puts "Dry run: no files, commits, tags, or remote branches changed."
  else
    git.call("add", "--", relative)
    puts git.call("commit", "-m", "chore(ios): release #{version} build #{build}")
    git.call("tag", "-a", tag, "-m", "NaKube iOS #{version} source build #{build}")
    begin
      puts git.call(*push)
    rescue RuntimeError => error
      raise "#{error.message}\nLocal release commit and tag were kept. Inspect them before retrying:\n  git #{push.shelljoin}"
    end
    puts "Released #{tag}; Xcode Cloud will build when its ios-v trigger is configured."
  end
  puts "Xcode Cloud uses MARKETING_VERSION plus its own CI_BUILD_NUMBER for TestFlight."
rescue OptionParser::ParseError, RuntimeError, SystemCallError => error
  warn "error: #{error.message}"
  exit 1
end
RUBY
