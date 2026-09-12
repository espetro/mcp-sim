---
tags:
- atd
- android
- smoke_test
- mcp_sim
---
Live ATD boot smoke results (2026-09-12, issue #34):

Setup: SDK root at /Volumes/KeVagiBe/android-sdk (main SSD only had ~11GB;
ATD image android-33 aosp_atd arm64-v8a + emulator 37.1.11 + platform-tools).
cmdline-tools copied from mise install (~/.local/share/mise/installs/
android-sdk/23.0). AVD `atd33` in ~/.android/avd with tag.id=aosp_atd,
hw.ramSize lowered to 1024 in config.ini (emulator logs "Increasing RAM
size to 2048MB" anyway; -memory 1536 honored on cmdline).

Results:
- Boot completed in ~22s (log) / <60s to `sys.boot_completed=1` (Android 13).
- qemu RSS settles ~1545 MB. 8GB host handles it (with ~2.2GB free after).
- Full MCP flow green: boot_device → state running with `atd:true,
  est_ram_mb:1536`; await_ready ready; open_url Success; get_state running;
  stream_info returns scrcpy guidance. tools/list = 11 tools.
- Caught and fixed a REAL crash: fatal concurrent map writes on
  platforms/android avdPortMap (MCP dispatches tool calls concurrently;
  get_state/await_ready raced boot_device). Fixed in d3f148c with portMu.

Gotchas:
- avdmanager (cmdline-tools 23) does NOT accept --sdk_root and silently
  reads ANDROID_HOME from env; sdkmanager does accept --sdk_root. Copying
  cmdline-tools into <SDK>/cmdline-tools/latest makes both work.
- avdmanager `create avd -k` accepts the package path only when it can
  load the local repo from ANDROID_HOME; "Package path is not valid" +
  "Valid system image paths are: null" means it looked at the wrong SDK.
- `emulator` must be on PATH for mcp-sim's android.New() LookPath check
  or the platform silently skips (only a WARN log). ANDROID_HOME alone
  is not enough for detection.
- Emulator instances die from AVD multiinstance.lock conflicts when two
  spawn against the same AVD; "no port mapping for AVD" from open_url in
  a parallel-call stream was this, not a map bug.
- On ATD there is no GUI/launcher; scrcpy would show nothing. screencap
  works for screenshots (docs/scrcpy.md correct).
