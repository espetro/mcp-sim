# Android ATD images

ATD (Automated Test Device) system images are Google's headless Android
images for automated testing. They skip the launcher, SystemUI and
hardware GPU paths, which makes each emulator instance markedly lighter
than a standard image: expect roughly 1.5 to 2 GB of host RAM per
instance on macOS, versus 3 to 4 GB for a full image.

## Requirements

- Android SDK with the emulator and cmdline-tools (sdkmanager, avdmanager)
- Supported images: `system-images;android-30..33;aosp_atd;arm64-v8a`
  (the highest ATD API level is 33; there is no android-34 ATD image)
- On Apple Silicon the arm64-v8a ATD images run natively under
  Hypervisor.framework, no x86 emulation involved

## Configuration

```yaml
platforms:
  android:
    enabled: true
    image_tag: aosp_atd   # aosp_atd | google_atd | default
    api: 33               # 30..33
    abi: arm64-v8a        # defaults per host arch
    ram_size: 1536        # MB passed to the emulator as -memory
    heap_size: 192        # MB, vm.heapSize in the AVD
    auto_provision: true  # install image + create AVD at boot time
```

Every field has an `MCPSIM_ANDROID_*` environment variable equivalent
(`MCPSIM_ANDROID_IMAGE_TAG`, `MCPSIM_ANDROID_API`, `MCPSIM_ANDROID_ABI`,
`MCPSIM_ANDROID_RAM_SIZE`, `MCPSIM_ANDROID_HEAP_SIZE`,
`MCPSIM_ANDROID_AUTO_PROVISION`).

## Auto provisioning

With `auto_provision: true`, booting a target whose AVD does not exist
runs:

1. `sdkmanager --install "system-images;android-<api>;aosp_atd;<abi>"`
2. `avdmanager create avd -n <target> -k <image> -d pixel`

The first install downloads a few hundred MB; subsequent boots reuse the
local image.

## Launch behavior

When the target AVD's `config.ini` contains `tag.id=aosp_atd` (or
`google_atd`) or an `_atd` image path, mcp-sim appends the ATD flags at
boot:

```
-no-window -no-audio -no-boot-anim -no-snapshot -gpu swiftshader_indirect
-memory <ram_size>
```

`list_devices` and `get_state` annotate ATD devices with `atd: true` and
an estimated RAM footprint (`est_ram_mb`) read from the AVD config, so
agents can pick lightweight targets without booting them first.

## Mirroring caveat

scrcpy mirroring is unreliable on ATD images: SystemUI is stubbed as
`com.android.fakesystemapp` and there is no hardware GPU. For visual QA
on ATD, use screenshots instead:

```
adb -s <serial> exec-out screencap -p > screen.png
```

scrcpy remains the recommended mirroring tool for standard images. See
[scrcpy.md](scrcpy.md).
