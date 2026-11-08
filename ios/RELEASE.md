# iOS releases

The iOS release lane is independent of the desktop packaging script
[`scripts/release.sh`](../scripts/release.sh). iOS tags begin with `ios/v`, never
bare `v`; do not add them to a desktop GoReleaser trigger. The
[iOS GitHub workflow](../.github/workflows/ios.yml) compiles the app but does not
sign, upload, or create a GitHub Release. Xcode Cloud owns signed archives.

## Prepare a release

[`scripts/bump-ios-version.sh`](../scripts/bump-ios-version.sh) updates the
project version and build number, then prints the exact Git commands for the
operator to review. It never commits, tags, or pushes.
Run it from a checkout containing the changes intended for release, with no
unrelated staged changes, and fetch remote tags before choosing a version.

```sh
scripts/bump-ios-version.sh --dry-run 1.2.3
scripts/bump-ios-version.sh 1.2.3
scripts/check-swift.sh
```

Review the version diff and checks, then use the printed commands. Push only the
explicit iOS tag, not `--tags` or `--follow-tags`, so an unrelated desktop tag
cannot start a desktop release. A version release uses `ios/v1.2.3`. To release
another source build of the same marketing version:

```sh
scripts/bump-ios-version.sh --dry-run --build-only
scripts/bump-ios-version.sh --build-only
```

Build-only tags include the incremented source build, such as
`ios/v1.2.3-build.3`. The tag must point to the commit containing the version
settings, not to its parent. Repeated dry runs do not change any file.

### Source build versus distributed build

The project stores matching Debug/Release `MARKETING_VERSION` and
`CURRENT_PROJECT_VERSION` settings. The bump script increments the latter for
local builds and source/tag identity. It is **not a reservation of an Xcode
Cloud build number**.

Xcode Cloud automatically assigns the distributed `CFBundleVersion` from its
own increasing `CI_BUILD_NUMBER`. For example, tag `ios/v1.2.3-build.3` can produce
TestFlight `1.2.3 (42)`. Rebuilding the same tag receives another Cloud build
number. Keep Cloud numbering authoritative; do not overwrite it with the source
counter or use a tag suffix as the uploaded build number. The
[post-clone hook](NakubeRemote/ci_scripts/ci_post_clone.sh) validates tag identity
and logs both numbers without modifying either.

If this app already has distributed builds of the same marketing version,
choose an unused next Cloud build number in **App Store Connect → app → Xcode
Cloud → Settings → Build Number** before distribution. An Admin or App Manager
can change it. See Apple's
[build numbering guidance](https://developer.apple.com/documentation/xcode/setting-the-next-build-number-for-xcode-cloud-builds).

## Swift checks

[`scripts/check-swift.sh`](../scripts/check-swift.sh) requires macOS and full
Xcode 16 or newer with the iOS Simulator SDK installed. It compiles the shared
`NakubeRemote` scheme in Debug and Release, with complete Swift concurrency
checking and Swift warnings as errors. This is a compilation check, not an
on-device test, UI test, formatter, or archive/distribution check. No test target
is currently configured.

Select the Xcode installation available on your machine with `xcode-select`, or
set `DEVELOPER_DIR` for this command. The GitHub workflow uses the selected Xcode
on `macos-15` and prints its version rather than assuming a fixed installation
path. Simulator checks disable code signing only for those invocations; device
and Cloud builds keep automatic signing enabled.

## Required Xcode Cloud setup

Cloud workflow configuration and Apple account authorization live in Xcode/App
Store Connect, not in GitHub Actions YAML. They must be configured once by a
team member with the necessary permissions:

1. Open `ios/NakubeRemote/NakubeRemote.xcodeproj` in full Xcode. Select the shared
   `NakubeRemote` scheme. In **Signing & Capabilities**, keep automatic signing
   enabled and confirm the Apple team and the `io.gnas.nakube.remote` bundle ID
   match the intended App Store Connect app. If using a different team, update
   both target build configurations and use a bundle ID registered to that team.
2. Use **Product → Xcode Cloud → Create Workflow** to authorize this GitHub
   repository and associate it with the iOS app. Commit the shared scheme and
   executable `ci_scripts` hook with the project before starting a Cloud build.
3. Create a release workflow using this project and scheme. Select a supported
   Xcode/macOS environment (Xcode 16 or newer), add an **Archive** action for iOS
   using Release, and choose the intended distribution method. Do not inject
   `CODE_SIGNING_ALLOWED=NO` or copy the simulator command's signing settings.
4. Replace the default main-branch start condition with **Tag Changes → Custom
   Tags → Tags beginning with `ios/v`**. Do not choose Any Tag or a bare `v`
   prefix. Leave tag file/folder filters unrestricted so a build-only tag starts
   a build even when it reuses previously built source. A separate branch/PR
   validation workflow is optional; GitHub already runs simulator checks.
5. If distributing to testers, configure a **TestFlight** post-action and select
   the intended tester group. Complete App Store Connect's app/distribution
   prerequisites (including required app artwork and metadata) before enabling
   uploads. A successful unsigned GitHub build does not prove signing or
   TestFlight readiness. No Apple credentials are needed in GitHub Actions.
6. Push the release commit and its explicit `ios/v…` tag using the reviewed
   commands. In Cloud, confirm the post-clone log's marketing version/tag and
   the Archive result. For TestFlight, correlate the uploaded build with the
   Cloud run using `CI_BUILD_NUMBER`, not the source build counter.

Cloud discovers the hook beside the project automatically. See Apple's
[custom build script conventions](https://developer.apple.com/documentation/xcode/writing-custom-build-scripts)
and [tag start conditions](https://developer.apple.com/documentation/xcode/configuring-start-conditions).
