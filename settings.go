package pixellab

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	contracts "github.com/Herrscherd/herrscher-contracts"
)

const (
	settingAccounts     = "accounts"
	settingAccountsFile = "accounts_file"
	settingAsepriteDir  = "aseprite_dir"
	settingEndpoint     = "endpoint"
	settingDefaultSize  = "default_size"
	settingMaxImages    = "max_images"
	settingTimeout      = "timeout"

	defaultEndpoint  = "wss://api.pixellab.ai"
	defaultSize      = 64
	defaultMaxImages = 4
	defaultTimeout   = 10 * time.Minute
	maxSize          = 256
	maxGrid          = 8
	defaultFrames    = 8
	maxFrames        = 16
)

type settings struct {
	accounts     string
	accountsFile string
	asepriteDir  string
	endpoint     string
	defaultSize  int
	maxImages    int
	timeout      time.Duration
}

func settingDeclarations() []contracts.Setting {
	return []contracts.Setting{
		{Key: settingAccounts, Env: "PIXELLAB_ACCOUNTS", Help: helpAccounts},
		{Key: settingAccountsFile, Env: "PIXELLAB_ACCOUNTS_FILE", Help: helpAccountsFile},
		{Key: settingAsepriteDir, Env: "PIXELLAB_ASEPRITE_DIR", Help: helpAsepriteDir},
		{Key: settingEndpoint, Env: "PIXELLAB_ENDPOINT", Help: helpEndpoint, Default: defaultEndpoint},
		{Key: settingDefaultSize, Env: "PIXELLAB_DEFAULT_SIZE", Help: helpDefaultSize, Default: strconv.Itoa(defaultSize)},
		{Key: settingMaxImages, Env: "PIXELLAB_MAX_IMAGES", Help: helpMaxImages, Default: strconv.Itoa(defaultMaxImages)},
		{Key: settingTimeout, Env: "PIXELLAB_TIMEOUT", Help: helpTimeout, Default: defaultTimeout.String()},
	}
}

func parseSettings(cfg contracts.PluginConfig) (settings, error) {
	size, err := positiveSetting(cfg, settingDefaultSize, defaultSize, maxSize)
	if err != nil {
		return settings{}, err
	}
	images, err := positiveSetting(cfg, settingMaxImages, defaultMaxImages, 0)
	if err != nil {
		return settings{}, err
	}
	timeout, err := timeoutSetting(cfg)
	if err != nil {
		return settings{}, err
	}
	endpoint, err := endpointSetting(cfg)
	if err != nil {
		return settings{}, err
	}
	return settings{
		accounts:     strings.TrimSpace(cfg.Get(settingAccounts)),
		accountsFile: strings.TrimSpace(cfg.Get(settingAccountsFile)),
		asepriteDir:  strings.TrimSpace(cfg.Get(settingAsepriteDir)),
		endpoint:     endpoint,
		defaultSize:  size,
		maxImages:    images,
		timeout:      timeout,
	}, nil
}

func positiveSetting(cfg contracts.PluginConfig, key string, fallback, limit int) (int, error) {
	text := strings.TrimSpace(cfg.Get(key))
	if text == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 1 || (limit > 0 && value > limit) {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidSetting, key, text)
	}
	return value, nil
}

func timeoutSetting(cfg contracts.PluginConfig) (time.Duration, error) {
	text := strings.TrimSpace(cfg.Get(settingTimeout))
	if text == "" {
		return defaultTimeout, nil
	}
	value, err := time.ParseDuration(text)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidSetting, settingTimeout, text)
	}
	return value, nil
}

func endpointSetting(cfg contracts.PluginConfig) (string, error) {
	text := strings.TrimSpace(cfg.Get(settingEndpoint))
	if text == "" {
		return defaultEndpoint, nil
	}
	parsed, err := url.Parse(text)
	if err != nil || !endpointAllowed(parsed) {
		return "", fmt.Errorf("%w: %s=%q, want a wss URL, or a ws URL on loopback", ErrInvalidSetting, settingEndpoint, text)
	}
	return strings.TrimSuffix(text, "/"), nil
}

func endpointAllowed(parsed *url.URL) bool {
	switch parsed.Scheme {
	case "wss":
		return parsed.Host != ""
	case "ws":
		return isLoopbackHost(parsed.Hostname())
	}
	return false
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
