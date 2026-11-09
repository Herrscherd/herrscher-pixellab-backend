package pixellab

import (
	"errors"
	"testing"

	contracts "github.com/Herrscherd/herrscher-contracts"
)

func TestParseSettingsDefaults(t *testing.T) {
	s, err := parseSettings(contracts.PluginConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if s.endpoint != defaultEndpoint || s.defaultSize != defaultSize || s.maxImages != defaultMaxImages || s.timeout != defaultTimeout {
		t.Fatalf("settings = %+v, want the defaults", s)
	}
}

func TestParseSettingsRejectsBadValues(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{name: "size zero", key: settingDefaultSize, value: "0"},
		{name: "size above the limit", key: settingDefaultSize, value: "512"},
		{name: "size not a number", key: settingDefaultSize, value: "big"},
		{name: "max images zero", key: settingMaxImages, value: "0"},
		{name: "timeout not a duration", key: settingTimeout, value: "soon"},
		{name: "timeout zero", key: settingTimeout, value: "0s"},
		{name: "endpoint over http", key: settingEndpoint, value: "https://api.pixellab.ai"},
		{name: "endpoint without host", key: settingEndpoint, value: "wss://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSettings(contracts.PluginConfig{Settings: map[string]string{tc.key: tc.value}})
			if !errors.Is(err, ErrInvalidSetting) {
				t.Fatalf("error = %v, want ErrInvalidSetting", err)
			}
		})
	}
}
