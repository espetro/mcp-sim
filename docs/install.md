# Installing and launching apps

`install_app` and `launch_app` get a caller-built artifact onto a booted
device and start it. mcp-sim never builds, signs, or transfers artifacts:
where the artifact comes from (xcodebuild, gradle, `eas build`, a CI cache)
is the caller's job. These tools are thin wrappers over `xcrun simctl
install/launch` and `adb install -r` / `am start`.

## install_app

```
install_app(platform, target, artifact_ref)
```

- iOS: `xcrun simctl install <target> <path>` with an `.app` bundle.
- Android: `adb -s <serial> install -r <path>` with an `.apk`.

The serial for Android is derived from the AVD's port mapping when the
emulator was started by mcp-sim; otherwise `target` is used directly, so
already-running emulators (`emulator-5554`) and physical devices work.

## artifact_ref forms

Three forms are accepted:

1. **Absolute path**: `/Users/runner/work/build/MyApp.app` — used as-is.
2. **Relative path**: `build/MyApp.app` — joined under the first
   `MCPSIM_ARTIFACT_ROOTS` entry where the file exists.
3. **`artifact://<name>/<rel>` URI**: joined under the root registered as
   `<name>`. Falls back to matching `<rel>` against every root (anonymous
   form) if no name matches and the ref exists under one.

## MCPSIM_ARTIFACT_ROOTS

Colon-separated, like `PATH`. Each entry is either an anonymous absolute
path or `name=/abs/path` for the `artifact://` scheme:

```bash
export MCPSIM_ARTIFACT_ROOTS="\
derived=$HOME/Library/Developer/Xcode/DerivedData,\
:name=/opt/ci/android-outputs"
```

(Entries are colon-separated; the line continuations above are only for
readability. On Windows `;` is the separator's natural neighbor, but the
server splits on `:` exactly like `PATH` on Unix; prefer paths without `:`.)

The same registry is portable across environments: locally
`derived=` points at DerivedData, on CI it points at the deploy directory.
A Bitrise step might call:

```
install_app(platform="ios", target="<udid>", artifact_ref="artifact://derived/MyApp/Build/Products/Debug-iphonesimulator/MyApp.app")
launch_app(platform="ios", target="<udid>", bundle_id="com.example.myapp")
```

Refs that resolve under no root fail with
`artifact: ref did not resolve under any MCPSIM_ARTIFACT_ROOTS entry`.

## launch_app

```
launch_app(platform, target, bundle_id) -> { "pid": <int> }
```

- iOS: `xcrun simctl launch <target> <bundle_id>`, pid parsed from output.
- Android: the launch activity is resolved with
  `adb shell cmd package resolve-activity --brief <bundle_id>` and started
  with `am start -n`; the pid is read back via `pidof` (0 when unavailable).

## What these tools do NOT do

- No build tool. Use XcodeBuildMCP, DroidPilot, gradle, `xcodebuild` in CI.
- No transfer tool. Use rsync/scp/curl to place artifacts on the host.
- No uninstall. Deliberately deferred until a concrete CI need appears.
- No remote artifact resolution (HTTP, s3). Download to disk first.
