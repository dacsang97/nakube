# iOS releases

The iOS release lane is independent of the desktop packaging script
[`scripts/release.sh`](../scripts/release.sh). iOS tags begin with `ios-v`, never
bare `v`; do not add them to a desktop GoReleaser trigger. The
[iOS GitHub workflow](../.github/workflows/ios.yml) checks shell script syntax
only; it does not compile, sign, upload, or create a GitHub Release.
Xcode Cloud owns release builds and signed archives.

## Prepare a release

[`scripts/bump-ios-version.sh`](../scripts/bump-ios-version.sh) updates the
project version and build number, commits only the project version settings,
creates an annotated iOS tag, and atomically pushes `main` and that tag to
`origin`. It requires a clean `main`, configured Git identity, an accessible
`origin/main`, no missing remote commits, and an unused tag. Local commits ahead
of `origin/main` are included in the release. It fetches the remote branch and
tags before changing files.

Optionally compile locally before releasing; the release command does not build the app:

```sh
scripts/check-swift.sh
scripts/bump-ios-version.sh --dry-run 1.2.3
scripts/bump-ios-version.sh 1.2.3
```

`--dry-run` performs the same preflight checks and fetch, but does not edit
files, commit, create tags, or push. The push explicitly includes only the
release branch and iOS tag, even when Git's `push.followTags` is enabled.
A version release uses `ios-v1.2.3`. To release another source build of the
same marketing version:

```sh
scripts/bump-ios-version.sh --dry-run --build-only
scripts/bump-ios-version.sh --build-only
```

Build-only tags include the incremented source build, such as
`ios-v1.2.3-build.3`. The tag must point to the commit containing the version
settings, not to its parent. Repeated dry runs do not change any file.

If the atomic push fails, no branch/tag pair is partially published. The local
release commit and tag remain for inspection; use the retry command printed by
the script after resolving the failure. Do not rerun the bump command, which
would select another source build or reject the existing version tag.

### Source build versus distributed build

The project stores matching Debug/Release `MARKETING_VERSION` and
`CURRENT_PROJECT_VERSION` settings. The bump script increments the latter for
local builds and source/tag identity. It is **not a reservation of an Xcode
Cloud build number**.

Xcode Cloud automatically assigns the distributed `CFBundleVersion` from its
own increasing `CI_BUILD_NUMBER`. For example, tag `ios-v1.2.3-build.3` can produce
TestFlight `1.2.3 (42)`. Rebuilding the same tag receives another Cloud build
number. Keep Cloud numbering authoritative; do not overwrite it with the source
counter or use a tag suffix as the uploaded build number.

If this app already has distributed builds of the same marketing version,
choose an unused next Cloud build number in **App Store Connect → app → Xcode
Cloud → Settings → Build Number** before distribution. An Admin or App Manager
can change it. See Apple's
[build numbering guidance](https://developer.apple.com/documentation/xcode/setting-the-next-build-number-for-xcode-cloud-builds).

## Optional local Swift checks

[`scripts/check-swift.sh`](../scripts/check-swift.sh) requires macOS and full
Xcode 16 or newer with the iOS Simulator SDK installed. It compiles the shared
`NakubeRemote` scheme in Debug and Release, with complete Swift concurrency
checking and Swift warnings as errors. This is a compilation check, not an
on-device test, UI test, formatter, or archive/distribution check. No test target
is currently configured.

GitHub CI does not run this compilation script. Its iOS workflow only checks
shell syntax on Ubuntu; release compilation is handled by Xcode Cloud.

Select the Xcode installation available on your machine with `xcode-select`, or
set `DEVELOPER_DIR` for this command. Local simulator checks disable code signing
only for those invocations; device and Cloud builds keep automatic signing enabled.

## Required Xcode Cloud setup

Cloud workflow configuration and Apple account authorization live in Xcode/App
Store Connect, not in GitHub Actions YAML. They must be configured once by a
team member with the necessary permissions:

1. Open `ios/NakubeRemote/NakubeRemote.xcodeproj` in full Xcode. Select the shared
   `NakubeRemote` scheme. In **Signing & Capabilities**, keep automatic signing
   enabled and confirm the Apple team and the `io.gnas.nakube.mobile` bundle ID
   match the intended App Store Connect app. If using a different team, update
   both target build configurations and use a bundle ID registered to that team.
2. Use **Product → Xcode Cloud → Create Workflow** to authorize this GitHub
   repository and associate it with the iOS app. Commit the shared scheme with
   the project before starting a Cloud build.
3. Create a release workflow using this project and scheme. Select a supported
   Xcode/macOS environment (Xcode 16 or newer), add an **Archive** action for iOS
   using Release, and choose the intended distribution method. Do not inject
   `CODE_SIGNING_ALLOWED=NO` or copy the simulator command's signing settings.
4. Replace the default main-branch start condition with **Tag Changes → Custom
   Tags → Tags beginning with `ios-v`**. Do not choose Any Tag or a bare `v`
   prefix. Leave tag file/folder filters unrestricted so a build-only tag starts
   a build even when it reuses previously built source. Configure a separate
   Xcode Cloud branch/PR validation workflow if pre-release compilation is needed.
5. If distributing to testers, configure a **TestFlight** post-action and select
   the intended tester group. Complete App Store Connect's app/distribution
   prerequisites (including required app artwork and metadata) before enabling
   uploads. A successful unsigned GitHub build does not prove signing or
   TestFlight readiness. No Apple credentials are needed in GitHub Actions.
6. Run the release script on clean `main` to push the release commit and its
   explicit `ios-v…` tag. In Cloud, confirm the selected tag and the Archive result. For
   TestFlight, correlate the uploaded build with the Cloud run using
   `CI_BUILD_NUMBER`, not the source build counter.

See Apple's [tag start conditions](https://developer.apple.com/documentation/xcode/configuring-start-conditions).
