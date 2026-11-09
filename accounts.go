package pixellab

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultTier       = 1
	asepriteExtension = "pixellab"
	asepriteManifest  = "package.json"
)

type account struct {
	secret  string
	version string
	tier    int
}

type asepriteCredentials struct {
	Version string `json:"version"`
	Secret  string `json:"secret"`
	Tier    int    `json:"tier"`
}

func loadAccounts(config settings, home string) ([]account, error) {
	fallback, found, err := asepriteAccount(config.asepriteDir, home)
	if err != nil {
		return nil, err
	}
	text, err := accountsText(config)
	if err != nil {
		return nil, err
	}
	accounts, err := parseAccounts(text, fallback)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 && found {
		accounts = []account{fallback}
	}
	if len(accounts) == 0 {
		return nil, ErrNoAccount
	}
	for index, entry := range accounts {
		if entry.version == "" {
			return nil, fmt.Errorf("%w: account %d has no version and no Aseprite extension to borrow one from", ErrInvalidAccount, index+1)
		}
	}
	return accounts, nil
}

func accountsText(config settings) (string, error) {
	if config.accounts != "" {
		return config.accounts, nil
	}
	if config.accountsFile == "" {
		return "", nil
	}
	raw, err := os.ReadFile(config.accountsFile)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrInvalidSetting, settingAccountsFile, err)
	}
	return string(raw), nil
}

func parseAccounts(text string, fallback account) ([]account, error) {
	var accounts []account
	lines := strings.FieldsFunc(text, func(char rune) bool { return char == '\n' || char == ';' })
	for _, line := range lines {
		before, _, _ := strings.Cut(line, "#")
		body := strings.TrimSpace(before)
		if body == "" {
			continue
		}
		entry, err := accountFromFields(strings.Split(body, "|"), fallback)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, entry)
	}
	return accounts, nil
}

func accountFromFields(fields []string, fallback account) (account, error) {
	secret := strings.TrimSpace(fields[0])
	if secret == "" {
		return account{}, fmt.Errorf("%w: a line has no secret", ErrInvalidAccount)
	}
	entry := account{secret: secret, version: fallback.version, tier: defaultTier}
	if fallback.tier > 0 {
		entry.tier = fallback.tier
	}
	if len(fields) > 1 && strings.TrimSpace(fields[1]) != "" {
		entry.version = strings.TrimSpace(fields[1])
	}
	if len(fields) > 2 && strings.TrimSpace(fields[2]) != "" {
		tier, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil || tier < 1 {
			return account{}, fmt.Errorf("%w: tier %q is not a positive number", ErrInvalidAccount, fields[2])
		}
		entry.tier = tier
	}
	return entry, nil
}

func asepriteAccount(dir, home string) (account, bool, error) {
	if dir == "" {
		if home == "" {
			return account{}, false, nil
		}
		dir = filepath.Join(home, ".config", "aseprite", "extensions")
	}
	raw, err := os.ReadFile(filepath.Join(dir, asepriteExtension, asepriteManifest))
	if errors.Is(err, os.ErrNotExist) {
		return account{}, false, nil
	}
	if err != nil {
		return account{}, false, fmt.Errorf("%w: %s: %w", ErrInvalidSetting, settingAsepriteDir, err)
	}
	var credentials asepriteCredentials
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return account{}, false, fmt.Errorf("%w: %s: %w", ErrInvalidAccount, asepriteManifest, err)
	}
	if credentials.Secret == "" {
		return account{}, false, nil
	}
	tier := credentials.Tier
	if tier < 1 {
		tier = defaultTier
	}
	return account{secret: credentials.Secret, version: credentials.Version, tier: tier}, true, nil
}
