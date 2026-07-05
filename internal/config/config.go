package config

import (
	"fmt"
	"os"
	"strconv"

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
	Listen    string `yaml:"listen"`     // MCPSIM_LISTEN
	LogLevel  string `yaml:"log_level"`  // MCPSIM_LOG_LEVEL
	LogFormat string `yaml:"log_format"` // MCPSIM_LOG_FORMAT
}

// PlatformsConfig holds per-platform configuration.
type PlatformsConfig struct {
	IOS     IOSConfig     `yaml:"ios"`
	Android AndroidConfig `yaml:"android"`
}

// IOSConfig configures the iOS platform adapter.
type IOSConfig struct {
	Enabled      bool   `yaml:"enabled"`       // MCPSIM_IOS_ENABLED
	DeveloperDir string `yaml:"developer_dir"` // MCPSIM_DEVELOPER_DIR
}

// AndroidConfig configures the Android platform adapter.
type AndroidConfig struct {
	Enabled     bool   `yaml:"enabled"`      // MCPSIM_ANDROID_ENABLED
	AndroidHome string `yaml:"android_home"` // MCPSIM_ANDROID_HOME
	JavaHome    string `yaml:"java_home"`    // MCPSIM_JAVA_HOME
	EmulatorBin string `yaml:"emulator_bin"` // MCPSIM_ANDROID_EMULATOR_BIN
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
	return Config{
		Server: ServerConfig{
			Listen:    ":9090",
			LogLevel:  "info",
			LogFormat: "text",
		},
		Platforms: PlatformsConfig{
			IOS: IOSConfig{
				Enabled: true,
			},
			Android: AndroidConfig{
				Enabled: true,
			},
		},
		Controllers: ControllersConfig{
			AgentDevice: AgentDeviceConfig{
				Enabled:   true,
				ProxyPort: 9000,
			},
		},
	}
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
	} else if home := os.Getenv("HOME"); home != "" {
		defaultPath := home + "/.config/mcp-sim/config.yaml"
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
	if v := os.Getenv("MCPSIM_IOS_ENABLED"); v != "" {
		cfg.Platforms.IOS.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_DEVELOPER_DIR"); v != "" {
		cfg.Platforms.IOS.DeveloperDir = v
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
	if v := os.Getenv("MCPSIM_AGENT_DEVICE_ENABLED"); v != "" {
		cfg.Controllers.AgentDevice.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("MCPSIM_AGENT_DEVICE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Controllers.AgentDevice.ProxyPort = port
		}
	}

	return cfg, nil
}

func loadFile(path string, cfg *Config) error {
	// #nosec G703 -- path comes from operator-controlled env or known fixed default
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
