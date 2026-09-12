Mini base setup complete + POC rerun green (issue #36, 2026-09-13):

quinos-mac-mini state: Xcode + iOS 26.5 (11 sims), simslim 0.8.0 (brew),
Android SDK at ~/Library/Android/sdk (cmdline-tools/latest layout,
emulator 37.1.11, platform-tools, ATD android-33 aosp_atd arm64-v8a),
AVD atd33 (tag.id=aosp_atd), JAVA_HOME=~/.local/share/mise/installs/java/17.0.2
(java NOT on default PATH, set it for sdkmanager/avdmanager).
Disk cost ~2.5GB. Serve left RUNNING on :9090 (leave it).

POC results: healthz 200, 401 challenge without token, 11 tools, 11 iOS +
1 Android devices, ATD boot + stays running (qemu RSS ~2.8GB on mini),
stream_info ok. Emulator killed cleanly after tests, serve still up.

TWO REAL BUGS found by the remote rerun, fixed in caeb89c:
1. CRITICAL: Start() bound the emulator process to the HTTP request ctx
   via exec.CommandContext; when boot_device responded, ctx cancel KILLED
   qemu. Local testing never caught it because... it should have (same
   path!) - earlier local runs survived by luck (stdio transport? no:
   previous local tests used stdio `mcp` mode where ctx outlives, or the
   pipeline's stdin-held ctx). Always test over HTTP serve mode.
2. State() leaked raw 'exit status 1' from cold adb server and aborted
   Boot before Start; now treated as stopped.

simslim only activates with MCPSIM_IOS_SLIM_ENABLED=true (by design);
get_state optimizer block absent when off, no warnings - correct.
