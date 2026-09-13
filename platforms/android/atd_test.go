package android

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadAVDTag(t *testing.T) {
	t.Parallel()

	writeConfig := func(t *testing.T, lines string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.avd", "config.ini")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	cases := []struct {
		name    string
		config  string
		want    string
		missing bool
	}{
		{
			name:   "aosp_atd tag.id",
			config: "image.sysdir.1=system-images\\android-33\\aosp_atd\\arm64-v8a\\\ntag.id=aosp_atd\n",
			want:   "aosp_atd",
		},
		{
			name:   "google_atd tag.id",
			config: "image.sysdir.1=system-images\\android-33\\google_atd\\arm64-v8a\\\ntag.id=google_atd\n",
			want:   "google_atd",
		},
		{
			name:   "default tag is not ATD",
			config: "image.sysdir.1=system-images\\android-33\\default\\arm64-v8a\\\ntag.id=default\n",
			want:   "",
		},
		{
			name:   "google_apis tag is not ATD",
			config: "tag.id=google_apis\n",
			want:   "",
		},
		{
			name:   "atd in image path without tag.id",
			config: "image.sysdir.1=system-images\\android-30\\aosp_atd\\x86_64\\\n",
			want:   "aosp_atd",
		},
		{
			name:    "missing file",
			want:    "",
			missing: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "does-not-exist.ini")
			if !tc.missing {
				path = writeConfig(t, tc.config)
			}
			if got := readAVDTag(path); got != tc.want {
				t.Errorf("readAVDTag() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAtdFlags(t *testing.T) {
	t.Parallel()

	got := atdFlags(1536)
	want := []string{
		"-no-window", "-no-audio", "-no-boot-anim", "-no-snapshot",
		"-gpu", "swiftshader_indirect",
		"-memory", "1536",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("atdFlags() = %v, want %v", got, want)
	}
	if !strings.Contains(strings.Join(got, " "), "-memory 1536") {
		t.Errorf("atdFlags missing RAM size: %v", got)
	}
}

func TestAvdConfigPath(t *testing.T) {
	t.Run("explicit avd home", func(t *testing.T) {
		got := avdConfigPath("/tmp/avds", "pixel")
		want := filepath.Join("/tmp/avds", "pixel.avd", "config.ini")
		if got != want {
			t.Errorf("avdConfigPath() = %q, want %q", got, want)
		}
	})
	t.Run("fallback to home", func(t *testing.T) {
		t.Setenv("ANDROID_AVD_HOME", "")
		home, _ := os.UserHomeDir()
		got := avdConfigPath("", "pixel")
		want := filepath.Join(home, ".android", "avd", "pixel.avd", "config.ini")
		if home != "" && got != want {
			t.Errorf("avdConfigPath() = %q, want %q", got, want)
		}
	})
}
