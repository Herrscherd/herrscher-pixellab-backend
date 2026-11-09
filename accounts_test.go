package pixellab

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAccountsReadsTheFormat(t *testing.T) {
	fallback := account{secret: "local", version: "0.4.100", tier: 2}
	cases := []struct {
		name    string
		text    string
		secrets []string
		version []string
		tier    []int
	}{
		{name: "bare secret borrows the fallback", text: "alpha", secrets: []string{"alpha"}, version: []string{"0.4.100"}, tier: []int{2}},
		{name: "inline version and tier", text: "beta | 0.4.120 | 3", secrets: []string{"beta"}, version: []string{"0.4.120"}, tier: []int{3}},
		{name: "comments and blank lines are skipped", text: "# head\n\ngamma # note\n", secrets: []string{"gamma"}, version: []string{"0.4.100"}, tier: []int{2}},
		{name: "semicolons separate accounts", text: "one | 1.0.0;two | 1.0.1", secrets: []string{"one", "two"}, version: []string{"1.0.0", "1.0.1"}, tier: []int{2, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accounts, err := parseAccounts(tc.text, fallback)
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != len(tc.secrets) {
				t.Fatalf("accounts = %+v, want %d", accounts, len(tc.secrets))
			}
			for index, acc := range accounts {
				if acc.secret != tc.secrets[index] || acc.version != tc.version[index] || acc.tier != tc.tier[index] {
					t.Fatalf("account %d = %+v, want %s %s %d", index, acc, tc.secrets[index], tc.version[index], tc.tier[index])
				}
			}
		})
	}
}

func TestParseAccountsRejectsABadTier(t *testing.T) {
	_, err := parseAccounts("alpha | 0.4.120\nbeta | 0.4.120 | gold-secret", account{})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("error = %v, want ErrInvalidAccount", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %v, want the line number", err)
	}
	if strings.Contains(err.Error(), "gold-secret") {
		t.Fatalf("error leaks the content of the line: %v", err)
	}
}

func TestLoadAccountsKeepsTheManifestOutOfErrors(t *testing.T) {
	dir := t.TempDir()
	extension := filepath.Join(dir, asepriteExtension)
	if err := os.MkdirAll(extension, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := "local-secret-value"
	if err := os.WriteFile(filepath.Join(extension, asepriteManifest), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadAccounts(settings{asepriteDir: dir}, "")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("error = %v, want ErrInvalidAccount", err)
	}
	if strings.Contains(err.Error(), "local-secret") {
		t.Fatalf("error leaks the manifest content: %v", err)
	}
}

func TestInvalidManifestErrorNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	extension := filepath.Join(dir, asepriteExtension)
	if err := os.MkdirAll(extension, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extension, asepriteManifest), []byte("local-secret-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadAccounts(settings{asepriteDir: dir}, "")
	if err == nil || !strings.Contains(err.Error(), asepriteManifest) {
		t.Fatalf("error = %v, want the manifest name in it", err)
	}
}

func TestLoadAccountsRequiresAVersion(t *testing.T) {
	_, err := loadAccounts(settings{accounts: "alpha"}, t.TempDir())
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("error = %v, want ErrInvalidAccount", err)
	}
}

func TestLoadAccountsFallsBackToTheAsepriteExtension(t *testing.T) {
	dir := t.TempDir()
	extension := filepath.Join(dir, asepriteExtension)
	if err := os.MkdirAll(extension, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":"0.4.120","secret":"local-secret","tier":2}`
	if err := os.WriteFile(filepath.Join(extension, asepriteManifest), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadAccounts(settings{asepriteDir: dir}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].secret != "local-secret" || accounts[0].version != "0.4.120" || accounts[0].tier != 2 {
		t.Fatalf("accounts = %+v, want the Aseprite account", accounts)
	}
}

func TestLoadAccountsFollowsASymlinkedAccountsFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "accounts.txt")
	if err := os.WriteFile(target, []byte("linked-secret | 0.4.120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "accounts.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadAccounts(settings{accountsFile: link, asepriteDir: t.TempDir()}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].secret != "linked-secret" {
		t.Fatalf("accounts = %+v, want the account of the linked file", accounts)
	}
}

func TestLoadAccountsFollowsASymlinkedManifest(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), asepriteManifest)
	if err := os.WriteFile(manifest, []byte(`{"version":"0.4.120","secret":"linked-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	extension := filepath.Join(dir, asepriteExtension)
	if err := os.MkdirAll(extension, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(manifest, filepath.Join(extension, asepriteManifest)); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadAccounts(settings{asepriteDir: dir}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].secret != "linked-secret" {
		t.Fatalf("accounts = %+v, want the account of the linked manifest", accounts)
	}
}

func TestLoadAccountsWithNothingConfiguredIsAnError(t *testing.T) {
	_, err := loadAccounts(settings{asepriteDir: t.TempDir()}, "")
	if !errors.Is(err, ErrNoAccount) {
		t.Fatalf("error = %v, want ErrNoAccount", err)
	}
}

func TestLoadAccountsReadsTheConfiguredFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.txt")
	if err := os.WriteFile(path, []byte("file-secret | 0.4.120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadAccounts(settings{accountsFile: path, asepriteDir: t.TempDir()}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].secret != "file-secret" {
		t.Fatalf("accounts = %+v, want the file account", accounts)
	}
}
