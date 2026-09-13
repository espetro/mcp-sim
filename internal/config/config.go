package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all mcp-sim configuration.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Platforms   PlatformsConfig   `yaml:"platforms"`
	Controllers ControllersConfig `yaml:"controllers"`
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	Listen    string     `yaml:"listen"`     // MCPSIM_LISTEN
	LogLevel  string     `yaml:"log_level"`  // MCPSIM_LOG_LEVEL
	LogFormat string     `yaml:"log_format"` // MCPSIM_LOG_FORMAT
	Auth      AuthConfig `yaml:"auth"`
}

// AuthConfig configures HTTP bearer authentication. stdio is always auth free.
type AuthConfig struct {
	// Enabled is on by default; --insecure-no-auth / MCPSIM_INSECURE_NO_AUTH
	// turn it off (gated, see internal/auth gate).
	Enabled bool `yaml:"enabled"` // MCPSIM_INSECURE_NO_AUTH (inverted)
	// Token is a static bearer token; when empty one is generated on first
	// boot and persisted to ~/.config/mcp-sim/token.
	Token string `yaml:"token"` // MCPSIM_AUTH_TOKEN
}

// PlatformsConfig holds per-platform configuration.
type PlatformsConfig struct {
	IOS     IOSConfig     `yaml:"ios"`
	Android AndroidConfig `yaml:"android"`
}

// IOSConfig configures the iOS platform adapter.
type IOSConfig struct {
	Enabled      bool       `yaml:"enabled"`       // MCPSIM_IOS_ENABLED
	DeveloperDir string     `yaml:"developer_dir"` // MCPSIM_DEVELOPER_DIR
	Slim         SlimConfig `yaml:"slim"`
}

// SlimConfig configures the optional simslim integration (iOS only).
// Slimming is never on by default: it disables simulator daemons that some
// features (push, StoreKit, search) may need. See docs/simslim.md.
type SlimConfig struct {
	Enabled      bool     `yaml:"enabled"`       // MCPSIM_IOS_SLIM_ENABLED (probes simslim on PATH)
	OnBoot       bool     `yaml:"on_boot"`       // MCPSIM_IOS_SLIM_ON_BOOT: slim during boot_device when enabled
	Profile      string   `yaml:"profile"`       // MCPSIM_IOS_SLIM_PROFILE: path to profile JSON; empty = default slim
	Except       []string `yaml:"except"`        // MCPSIM_IOS_SLIM_EXCEPT: category IDs to keep
	Keep         []string `yaml:"keep"`          // MCPSIM_IOS_SLIM_KEEP: launchd labels to keep
	BootTimeout  string   `yaml:"boot_timeout"`  // MCPSIM_IOS_SLIM_BOOT_TIMEOUT: SIMSLIM_BOOT_TIMEOUT passthrough
	SpawnTimeout string   `yaml:"spawn_timeout"` // MCPSIM_IOS_SLIM_SPAWN_TIMEOUT: SIMSLIM_SPAWN_TIMEOUT passthrough
}

// AndroidConfig configures the Android platform adapter.
type AndroidConfig struct {
	Enabled     bool   `yaml:"enabled"`      // MCPSIM_ANDROID_ENABLED
	AndroidHome string `yaml:"android_home"` // MCPSIM_ANDROID_HOME
	JavaHome    string `yaml:"java_home"`    // MCPSIM_JAVA_HOME
	EmulatorBin string `yaml:"emulator_bin"` // MCPSIM_ANDROID_EMULATOR_BIN
	// ImageTag selects the system image tag family: "aosp_atd", "google_atd"
	// or "default" (the tag the AVD was created with). ATD images are lean
	// automated-test-device builds with much lower RAM needs.
	ImageTag string `yaml:"image_tag"` // MCPSIM_ANDROID_IMAGE_TAG
	// API is the Android API level (30-33 for ATD images).
	API int `yaml:"api"` // MCPSIM_ANDROID_API
	// ABI is the system image ABI (e.g. arm64-v8a on arm64 hosts).
	ABI string `yaml:"abi"` // MCPSIM_ANDROID_ABI
	// RAMSize is emulator RAM in MB; used with ATD images (-memory).
	RAMSize int `yaml:"ram_size"` // MCPSIM_ANDROID_RAM_SIZE
	// HeapSize is the emulator heap size in MB.
	HeapSize int `yaml:"heap_size"` // MCPSIM_ANDROID_HEAP_SIZE
	// AutoProvision provisions missing ATD AVDs via sdkmanager/avdmanager.
	AutoProvision bool `yaml:"auto_provision"` // MCPSIM_ANDROID_AUTO_PROVISION
}

// AllowedImageTags lists the valid values for AndroidConfig.ImageTag.
var AllowedImageTags = []string{"aosp_atd", "google_atd", "default"}

// Validate checks the Android config for semantic errors.
func (c AndroidConfig) Validate() error {
	if c.API != 0 && (c.API < 30 || c.API > 33) {
		return fmt.Errorf("android.api must be between 30 and 33, got %d", c.API)
	}
	if c.ImageTag != "" && !slices.Contains(AllowedImageTags, c.ImageTag) {
		return fmt.Errorf("android.image_tag must be one of %s, got %q", strings.Join(AllowedImageTags, ", "), c.ImageTag)
	}
	return nil
}

// ControllersConfig holds per-controller configuration.
type ControllersConfig struct {
	AgentDevice AgentDeviceConfig `yaml:"agentdevice"`
}

// AgentDeviceConfig configures the agent-device controller adapter.
type AgentDeviceConfig struct {
	Enabled   bool `yaml:"enabled"`    // MCPSIM_AGENT_DEVICE_ENABLED
	ProxyPort int  `yaml:"proxy_port"` // MCPSIM_AGENT_DEVICE_PORT
}

// Built-in defaults.
func defaultConfig() Config {
	cfg := Config{
		Server: ServerConfig{
			Listen:    ":9090",
			LogLevel:  "info",
			LogFormat: "text",
			Auth: AuthConfig{
				Enabled: true,
			},
		},
		Platforms: PlatformsConfig{
			IOS: IOSConfig{
				Enabled: true,
			},
			Android: AndroidConfig{
				Enabled:  true,
				ImageTag: "default",
				API:      33,
				RAMSize:  1536,
				HeapSize: 192,
			},
		},
	}
	if runtime.GOARCH == "arm64" {
		cfg.Platforms.Android.ABI = "arm64-v8a"
	} else {
		cfg.Platforms.Android.ABI = "x86_64"
	}
	return cfg
}

// Load reads config from: YAML file > env vars override > defaults.
// YAML path: ~/.config/mcp-sim/config.yaml or $MCPSIM_CONFIG.
func Load() (Config, error) {
	cfg := defaultConfig()

	// Load YAML file if present.
	if path := os.Getenv("MCPSIM_CONFIG"); path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return Config{}, fmt.Errorf("loading config from %s: %w", path, err)
		}
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		defaultPath := filepath.Join(home, ".config", "mcp-sim", "config.yaml")
		_ = loadFile(defaultPath, &cfg) // ignore error if missing
	}

	// Env var overrides (highest priority).
	if v := os.Getenv("MCPSIM_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("MCPSIM_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("MCPSIM_LOG_FORMAT"); v != "" {
		cfg.Server.LogFormat = v
	}
	if v := os.Getenv("MCPSIM_AUTH_TOKEN"); v != "" {
		cfg.Server.Auth.Token = v
	}
	if v := os.Getenv("MCPSIM_INSECURE_NO_AUTH"); v != "" {
		if disable, err := strconv.ParseBool(v); err == nil && disable {
			cfg.Server.Auth.Enabled = false
		}
	}
	if v := os.Getenv("MCPSIM_IOS_ENABLED"); v != "" {
		cfg.Platforms.IOS.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_DEVELOPER_DIR"); v != "" {
		cfg.Platforms.IOS.DeveloperDir = v
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_ENABLED"); v != "" {
		cfg.Platforms.IOS.Slim.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_ON_BOOT"); v != "" {
		cfg.Platforms.IOS.Slim.OnBoot, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_PROFILE"); v != "" {
		cfg.Platforms.IOS.Slim.Profile = v
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_EXCEPT"); v != "" {
		cfg.Platforms.IOS.Slim.Except = splitCSV(v)
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_KEEP"); v != "" {
		cfg.Platforms.IOS.Slim.Keep = splitCSV(v)
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_BOOT_TIMEOUT"); v != "" {
		cfg.Platforms.IOS.Slim.BootTimeout = v
	}
	if v := os.Getenv("MCPSIM_IOS_SLIM_SPAWN_TIMEOUT"); v != "" {
		cfg.Platforms.IOS.Slim.SpawnTimeout = v
	}
	if v := os.Getenv("MCPSIM_ANDROID_ENABLED"); v != "" {
		cfg.Platforms.Android.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_ANDROID_HOME"); v != "" {
		cfg.Platforms.Android.AndroidHome = v
	}
	if v := os.Getenv("MCPSIM_JAVA_HOME"); v != "" {
		cfg.Platforms.Android.JavaHome = v
	}
	if v := os.Getenv("MCPSIM_ANDROID_EMULATOR_BIN"); v != "" {
		cfg.Platforms.Android.EmulatorBin = v
	}
	if v := os.Getenv("MCPSIM_ANDROID_IMAGE_TAG"); v != "" {
		cfg.Platforms.Android.ImageTag = v
	}
	if v := os.Getenv("MCPSIM_ANDROID_API"); v != "" {
		if api, err := strconv.Atoi(v); err == nil {
			cfg.Platforms.Android.API = api
		}
	}
	if v := os.Getenv("MCPSIM_ANDROID_ABI"); v != "" {
		cfg.Platforms.Android.ABI = v
	}
	if v := os.Getenv("MCPSIM_ANDROID_RAM_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Platforms.Android.RAMSize = n
		}
	}
	if v := os.Getenv("MCPSIM_ANDROID_HEAP_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Platforms.Android.HeapSize = n
		}
	}
	if v := os.Getenv("MCPSIM_ANDROID_AUTO_PROVISION"); v != "" {
		cfg.Platforms.Android.AutoProvision, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_AGENT_DEVICE_ENABLED"); v != "" {
		cfg.Controllers.AgentDevice.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_AGENT_DEVICE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Controllers.AgentDevice.ProxyPort = port
		}
	}

	if err := cfg.Platforms.Android.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid android config: %w", err)
	}

	return cfg, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadFile(path string, cfg *Config) error {
	// #nosec G703 -- path comes from operator-controlled env or known fixed default
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
