# Redroid platform adapter (WIP)

Support for [Redroid](https://github.com/remote-android/redroid-doc) (Remote
Android in Docker) containers is work in progress and UNTESTED on real
hardware. The adapter was authored on macOS, where Redroid cannot run at all,
so it is validated only by compilation under `GOOS=linux` and by pure logic
unit tests. Do not rely on it until it has been exercised on a Linux host.
The adapter is Linux only and gated behind build tags, plus an opt in env
flag `MCPSIM_REDROID_ENABLED=1`.

## Host kernel requirements

Redroid needs Android binder IPC available in the host kernel. On stock
distro kernels install the extra module packages:

* Debian/Ubuntu: `linux-modules-extra-$(uname -r)` provides the `binder_linux`
  (binderfs) module. Load with `modprobe binder_linux devices="binder,hwbinder,vndbinder"`.
* Arch: `linux-headers` plus an AUR binder module or a custom kernel with
  `CONFIG_ANDROID_BINDER_IPC` / `CONFIG_ANDROID_BINDERFS` enabled.

Verify with `ls /dev/binderfs` or `grep binder /proc/filesystems`.

## Images and architectures

The adapter defaults to `redroid/redroid:12.0.0-latest_64only`. Use 64only
images on arm64 hosts; they avoid pulling 32 bit translation layers that do
not exist there. Override the image with `REDROID_IMAGE`. Plan roughly 1.5 to
2 GB of RAM per running instance plus container overhead.

## adb connect flow

Each container publishes its internal adb port 5555 on a free host port
(5555 and up). After `docker run`, the adapter runs:

```
adb connect localhost:<port>
adb -s localhost:<port> shell getprop sys.boot_completed   # wait for "1"
```

Subsequent commands (deep links via `am start`, state checks) use the same
serial. Only targets started by the current process are tracked, so a
restart of mcp-sim loses the port mapping for existing containers.

## Wipe semantics

Wipe and Stop both run `docker rm -f`. There is no persistent data volume:
every Start creates a fresh container from the image, which by definition has
fresh user data. If you need persistence, manage volumes yourself outside
this adapter.

## macOS non viability

Redroid cannot run under Docker Desktop on macOS because containers execute
in a Linux VM whose kernel lacks binder modules, and the VM kernel is not
user replaceable. See redroid-doc issues #165 and #867 for the long standing
requests. The apple/container project (issue #1737) is one to watch for a
future Linux container runtime with custom kernels on Apple silicon, but
nothing shippable today. Test Redroid on a native Linux host or a Linux VM
whose kernel you control.
