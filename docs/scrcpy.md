# scrcpy: on demand GUI mirroring

## What scrcpy is

[scrcpy](https://github.com/Genymobile/scrcpy) is an open source tool from Genymobile that mirrors and controls an Android device from a computer. It pushes a small server binary onto the device over adb, the device encodes its screen as an H.264 stream, and the desktop client decodes and displays it. It requires no root and no app installed on the device. Input (keyboard and mouse) is forwarded back over the same adb connection.

## When to use it

Use scrcpy only with **standard emulator images** (the regular `google_apis` or `default` system images). Standard images ship a real SystemUI and, on a capable host, access to hardware or emulated GPU encoders, so the video stream works reliably.

Do **not** use scrcpy with ATD (Automated Test Device) images. See the caveat below.

## Mirroring an emulator over TCP

The normal flow for a headless emulator (started with `-no-window`, for example by mcp-sim) is adb over TCP/IP plus `scrcpy --tcpip`:

1. Find the serial of the running AVD:
   ```bash
   adb devices
   ```
   Emulator serials look like `emulator-5554`.
2. Enable adb's TCP/IP listener on the device:
   ```bash
   adb -s emulator-5554 tcpip 5555
   ```
3. Connect to it (the emulator host is `localhost`, and the emulator maps the device's TCP port to a host port, typically `5555` for the first instance):
   ```bash
   adb connect localhost:5555
   ```
4. Mirror:
   ```bash
   scrcpy --tcpip=localhost:5555
   # or, once the device is already in adb devices:
   scrcpy -s localhost:5555
   ```
5. Disconnect when done:
   ```bash
   adb disconnect
   ```

Notes:

- `scrcpy --tcpip=<addr>` can do steps 2 and 3 itself when the device is reachable over USB, but for emulators the explicit `adb tcpip` then `adb connect` sequence above is the dependable path.
- If you select a device by serial, `scrcpy -s <serial>` works with the same serials adb uses; for TCP connections the serial is the `host:port` pair.
- The device and computer must be on the same network (for emulators, localhost suffices).

## The ATD caveat

ATD images are built for automated tests, not for humans looking at a screen:

- SystemUI is stubbed as `com.android.fakesystemapp`, so there is no real status bar, navigation bar, or window surface to mirror.
- ATD boots are configured without hardware GPU access (software rendering only), so the device-side video encoder scrcpy relies on is unreliable or absent.

Result: scrcpy mirroring of an ATD device is unreliable and should not be attempted. The GUI QA fallback on ATD is screenshots:

```bash
adb -s <serial> exec-out screencap -p > screen.png
```

This works on ATD and on standard images alike.

## Why mcp-sim does not embed streaming

mcp-sim is agent first. Agents consume screens as screenshots through the existing tools, which is deterministic, cheap, and works over plain adb with no extra host dependencies. Embedding an H.264 pipeline (scrcpy server push, decoding, frame serving) would add a large operational surface (ffmpeg, scrcpy-server version pinning, per frame latency budgets) that agents do not need.

Instead, mcp-sim treats scrcpy as an operator side tool:

- The `stream_info` MCP tool returns connection guidance (serial lookup, `adb tcpip 5555`, `adb connect`, `scrcpy --tcpip=<host>:5555`) for the device, so a human operator can attach a live mirror when doing visual verification.
- Agents continue using screenshot based workflows.
- Reference MCP projects that do embed streaming (mobile-mcp style servers, mcp-scrcpy-vision, scrcpy-mcp) exist if a project genuinely needs live video for an agent loop, but that is an application concern, not an emulator lifecycle concern.
