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
	if s.endpoint != defaultEndpoint || s.defaultSize != defaultSize || s.maxImages != defaultMaxImages || s.timeout != defaultTimeout || s.turnTimeout != defaultTurnTimeout {
		t.Fatalf("settings = %+v, want the defaults", s)
	}
}

func TestEndpointAcceptsWssAndLoopbackWs(t *testing.T) {
	for _, value := range []string{"wss://api.pixellab.ai", "ws://127.0.0.1:9000", "ws://localhost:9000", "ws://[::1]:9000"} {
		s, err := parseSettings(contracts.PluginConfig{Settings: map[string]string{settingEndpoint: value}})
		if err != nil || s.endpoint != value {
			t.Errorf("endpoint %q: got %q, err %v", value, s.endpoint, err)
		}
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
		{name: "turn timeout not a duration", key: settingTurnTimeout, value: "later"},
		{name: "turn timeout negative", key: settingTurnTimeout, value: "-1m"},
		{name: "endpoint over http", key: settingEndpoint, value: "https://api.pixellab.ai"},
		{name: "endpoint without host", key: settingEndpoint, value: "wss://"},
		{name: "ws on a public host", key: settingEndpoint, value: "ws://api.pixellab.ai"},
		{name: "ws on a private address", key: settingEndpoint, value: "ws://10.0.0.5:9000"},
		{name: "ws without host", key: settingEndpoint, value: "ws://"},
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
