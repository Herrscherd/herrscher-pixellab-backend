package pixellab

import (
	"errors"
	"io/fs"
	"slices"
	"testing"

	contracts "github.com/Herrscherd/herrscher-contracts"
	"github.com/Herrscherd/herrscher-contracts/i18n"
	"github.com/Herrscherd/herrscher-contracts/i18n/i18ntest"
)

func TestRegisterDeclaresABackend(t *testing.T) {
	var found *contracts.Plugin
	for _, plugin := range contracts.Default.Backends() {
		if plugin.Manifest.Kind == backendKind {
			found = &plugin
		}
	}
	if found == nil {
		t.Fatalf("no backend registered under %s", backendKind)
	}
	if found.Manifest.Category != contracts.CategoryBackend || found.Backend == nil || found.Probe == nil || found.Catalog == nil {
		t.Fatalf("manifest %+v, want a backend factory, a probe and a catalog", found.Manifest)
	}
	if _, err := found.Backend(t.Context(), contracts.PluginConfig{Settings: map[string]string{settingAccounts: "", settingAsepriteDir: t.TempDir()}}); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("factory without account error = %v, want ErrNoAccount", err)
	}
}

func TestProbeWithoutAccountNamesTheVariable(t *testing.T) {
	report := prober{}.probe(t.Context(), contracts.ProbeAccess{Config: contracts.PluginConfig{Settings: map[string]string{settingAsepriteDir: t.TempDir()}}})
	if report.State != contracts.HealthDown || report.Detail.Code != contracts.CodeHealthEnvMissing {
		t.Fatalf("probe = %+v, want down with the variable named", report)
	}
}

func TestProbeWithAccountIsConnected(t *testing.T) {
	cfg := contracts.PluginConfig{Settings: map[string]string{settingAccounts: "secret | 0.4.120"}}
	if report := (prober{}).probe(t.Context(), contracts.ProbeAccess{Config: cfg}); report.State != contracts.HealthOK {
		t.Fatalf("probe = %+v, want ok", report)
	}
}

func TestSettingHelpsAreCatalogKeys(t *testing.T) {
	for _, setting := range settingDeclarations() {
		if setting.Help == "" || setting.Help == setting.Key {
			t.Errorf("setting %s has help %q, want a catalog key", setting.Key, setting.Help)
		}
	}
}

func TestEveryMessageRendersInEveryLanguage(t *testing.T) {
	sources, err := fs.Sub(localeFiles, "locales")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := i18n.NewCatalog(nil, sources)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	keys := []string{
		helpAccounts, helpAccountsFile, helpAsepriteDir, helpEndpoint,
		helpDefaultSize, helpMaxImages, helpTimeout, helpTurnTimeout, replyDelivered, progressPercent,
	}
	params := map[string]any{"Count": 2, "Percent": 40}
	for _, lang := range i18n.Languages() {
		for _, key := range keys {
			text, err := catalog.Message(lang, key, params)
			if err != nil || text == key {
				t.Errorf("%s %s = %q, %v", lang, key, text, err)
			}
		}
	}
}

func TestLocalesShareTheirKeys(t *testing.T) {
	ids := func(name string) []string {
		raw, err := localeFiles.ReadFile("locales/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return catalogIDs(t, raw)
	}
	if english, french := ids("en.json"), ids("fr.json"); !slices.Equal(english, french) {
		t.Fatalf("en %v and fr %v differ", english, french)
	}
}

func TestCatalogPassesTheHouseCheck(t *testing.T) {
	sources, err := fs.Sub(localeFiles, "locales")
	if err != nil {
		t.Fatal(err)
	}
	i18ntest.Check(t, sources)
}
