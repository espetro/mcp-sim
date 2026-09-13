package android

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// avdConfigPath resolves the config.ini path for an AVD, honoring
// $ANDROID_AVD_HOME (also set by the emulator itself) and defaulting to
// ~/.android/avd/<name>.avd/config.ini.
func avdConfigPath(avdHome, name string) string {
	if avdHome == "" {
		avdHome = os.Getenv("ANDROID_AVD_HOME")
	}
	if avdHome == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			avdHome = filepath.Join(home, ".android", "avd")
		}
	}
	if avdHome == "" {
		return ""
	}
	return filepath.Join(avdHome, name+".avd", "config.ini")
}

// readAVDTag parses an AVD's config.ini and returns its image tag if the AVD
// uses an ATD (Automated Test Device) image. Returns "" for non-ATD AVDs or
// unreadable/missing files.
func readAVDTag(path string) string {
	data, err := os.Open(path) // #nosec G304 -- path derived from AVD home
	if err != nil {
		return ""
	}
	defer func() { _ = data.Close() }()

	tagID := ""
	imageSysdir := ""
	sc := bufio.NewScanner(data)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "tag.id":
			tagID = value
		case "image.sysdir.1":
			imageSysdir = value
		}
	}

	if strings.Contains(tagID, "_atd") {
		return tagID
	}
	if strings.Contains(imageSysdir, "_atd") {
		// Fall back to the tag embedded in the image path. config.ini uses
		// backslash separators on every platform.
		segments := strings.FieldsFunc(imageSysdir, func(r rune) bool {
			return r == '/' || r == '\\' || r == filepath.Separator
		})
		for _, seg := range segments {
			if strings.HasSuffix(seg, "_atd") {
				return seg
			}
		}
	}
	return ""
}

// atdFlags returns the emulator flags recommended for ATD images.
func atdFlags(ramSizeMB int) []string {
	return []string{
		"-no-window",
		"-no-audio",
		"-no-boot-anim",
		"-no-snapshot",
		"-gpu", "swiftshader_indirect",
		"-memory", fmt.Sprint(ramSizeMB),
	}
}

// provisionATDAvd creates an ATD AVD via sdkmanager + avdmanager. Only called
// when cfg.AutoProvision is set and the AVD does not exist yet.
func (p *Platform) provisionATDAvd(ctx context.Context, cfg avdSpec, name string) error {
	image := fmt.Sprintf("system-images;android-%d;%s;%s", cfg.api, cfg.tag, cfg.abi)

	sdkmanager := filepath.Join(p.androidHome, "cmdline-tools", "latest", "bin", "sdkmanager")
	if _, err := os.Stat(sdkmanager); err != nil {
		sdkmanager = filepath.Join(p.androidHome, "tools", "bin", "sdkmanager")
	}
	install := exec.CommandContext(ctx, sdkmanager, "--install", image) // #nosec G204 -- paths from config
	install.Env = p.env()
	if out, err := install.CombinedOutput(); err != nil {
		return fmt.Errorf("sdkmanager --install %s: %w: %s", image, err, strings.TrimSpace(string(out)))
	}

	avdmanager := filepath.Join(p.androidHome, "cmdline-tools", "latest", "bin", "avdmanager")
	if _, err := os.Stat(avdmanager); err != nil {
		avdmanager = filepath.Join(p.androidHome, "tools", "bin", "avdmanager")
	}
	create := exec.CommandContext(ctx, avdmanager, "create", "avd", "-n", name, "-k", image, "-d", "pixel") // #nosec G204 -- paths from config
	create.Env = p.env()
	if out, err := create.CombinedOutput(); err != nil {
		return fmt.Errorf("avdmanager create avd %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// avdSpec carries the resolved ATD provisioning parameters.
type avdSpec struct {
	api int
	tag string
	abi string
}

// resolveTargetTag returns the image tag of the target AVD, provisioning it
// first when it does not exist and AutoProvision is enabled. The second
// return value is non-nil when the tag could not be determined.
func (p *Platform) resolveTargetTag(ctx context.Context, target string) (string, error) {
	path := avdConfigPath("", target)
	if path == "" {
		return "", fmt.Errorf("cannot resolve AVD config path for %s", target)
	}
	if _, err := os.Stat(path); err != nil {
		if !p.autoProvision {
			return "", fmt.Errorf("AVD %s not found: %w", target, err)
		}
		tag := p.imageTag
		if tag == "" {
			tag = "aosp_atd"
		}
		api := p.api
		if api == 0 {
			api = 33
		}
		abi := p.abi
		if abi == "" {
			abi = "x86_64"
			if runtime.GOARCH == "arm64" {
				abi = "arm64-v8a"
			}
		}
		if err := p.provisionATDAvd(ctx, avdSpec{api: api, tag: tag, abi: abi}, target); err != nil {
			return "", err
		}
	}
	tag := readAVDTag(path)
	if tag == "" && strings.Contains(p.imageTag, "atd") {
		// Configured for ATD but the AVD predates the tag convention.
		return p.imageTag, nil
	}
	return tag, nil
}

// tagFor returns cached ATD annotations for an AVD name.
func (p *Platform) tagFor(name string) tagInfo {
	p.tagMu.Lock()
	defer p.tagMu.Unlock()
	if info, ok := p.avdTagCache[name]; ok {
		return info
	}
	tag := readAVDTag(avdConfigPath("", name))
	info := tagInfo{atd: strings.Contains(tag, "atd")}
	if info.atd {
		ram := p.ramSizeMB
		if ram == 0 {
			ram = 1536
		}
		info.estRAMMB = ram
	}
	p.avdTagCache[name] = info
	return info
}
