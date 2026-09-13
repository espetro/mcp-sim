package config

import (
	"runtime"
	"strings"
	"testing"
)

func TestAndroidValidateAPIBounds(t *testing.T) {
	t.Parallel()

	for _, api := range []int{0, 30, 31, 32, 33} {
		cfg := AndroidConfig{API: api}
		if err := cfg.Validate(); err != nil {
			t.Errorf("API %d should be valid, got error: %v", api, err)
		}
	}
	for _, api := range []int{-1, 29, 34, 100} {
		cfg := AndroidConfig{API: api}
		if err := cfg.Validate(); err == nil {
			t.Errorf("API %d should be invalid, got nil error", api)
		} else if !strings.Contains(err.Error(), "30") {
			t.Errorf("API %d error should mention bounds: %v", api, err)
		}
	}
}

func TestAndroidValidateImageTag(t *testing.T) {
	t.Parallel()

	for _, tag := range append([]string{""}, AllowedImageTags...) {
		cfg := AndroidConfig{ImageTag: tag}
		if err := cfg.Validate(); err != nil {
			t.Errorf("image_tag %q should be valid, got error: %v", tag, err)
		}
	}
	cfg := AndroidConfig{ImageTag: "bogus"}
	if err := cfg.Validate(); err == nil {
		t.Error("image_tag \"bogus\" should be invalid, got nil error")
	}
}

func TestLoadAndroidEnvOverrides(t *testing.T) {
	t.Setenv("MCPSIM_ANDROID_IMAGE_TAG", "aosp_atd")
	t.Setenv("MCPSIM_ANDROID_API", "30")
	t.Setenv("MCPSIM_ANDROID_ABI", "x86_64")
	t.Setenv("MCPSIM_ANDROID_RAM_SIZE", "2048")
	t.Setenv("MCPSIM_ANDROID_HEAP_SIZE", "256")
	t.Setenv("MCPSIM_ANDROID_AUTO_PROVISION", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	a := cfg.Platforms.Android
	if a.ImageTag != "aosp_atd" || a.API != 30 || a.ABI != "x86_64" || a.RAMSize != 2048 || a.HeapSize != 256 || !a.AutoProvision {
		t.Errorf("unexpected android config: %+v", a)
	}
}

func TestLoadAndroidInvalidEnv(t *testing.T) {
	t.Setenv("MCPSIM_ANDROID_API", "42")
	if _, err := Load(); err == nil {
		t.Error("Load() should reject android.api=42")
	}
}

func TestDefaultAndroidABIByGOARCH(t *testing.T) {
	cfg := defaultConfig()
	a := cfg.Platforms.Android
	if a.ImageTag != "default" || a.API != 33 || a.RAMSize != 1536 || a.HeapSize != 192 || a.AutoProvision {
		t.Errorf("unexpected android defaults: %+v", a)
	}
	switch {
	case runtime.GOARCH == "arm64" && a.ABI != "arm64-v8a":
		t.Errorf("arm64 host should default to arm64-v8a, got %q", a.ABI)
	case runtime.GOARCH != "arm64" && a.ABI != "x86_64":
		t.Errorf("non-arm64 host should default to x86_64, got %q", a.ABI)
	}
}
