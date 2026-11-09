package pixellab

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sync"

	"github.com/Herrscherd/herrscher-contracts/i18n"
)

//go:embed locales
var localeFiles embed.FS

const (
	helpAccounts     = "pixellab.setting.accounts"
	helpAccountsFile = "pixellab.setting.accounts_file"
	helpAsepriteDir  = "pixellab.setting.aseprite_dir"
	helpEndpoint     = "pixellab.setting.endpoint"
	helpDefaultSize  = "pixellab.setting.default_size"
	helpMaxImages    = "pixellab.setting.max_images"
	helpTimeout      = "pixellab.setting.timeout"
	helpTurnTimeout  = "pixellab.setting.turn_timeout"
	replyDelivered   = "pixellab.reply.delivered"
	progressPercent  = "pixellab.progress.percent"
)

type messages struct {
	load func() (*i18n.Catalog, error)
}

func newMessages() *messages {
	return &messages{load: sync.OnceValues(loadCatalog)}
}

func loadCatalog() (*i18n.Catalog, error) {
	sources, err := fs.Sub(localeFiles, "locales")
	if err != nil {
		return nil, fmt.Errorf("pixellab locales: %w", err)
	}
	return i18n.NewCatalog(nil, sources)
}

func (m *messages) render(lang i18n.Language, key string, params map[string]any) string {
	catalog, err := m.load()
	if err != nil {
		slog.Warn("pixellab: load messages", "key", key, "err", err)
		return key
	}
	text, err := catalog.Message(lang, key, params)
	if err != nil {
		slog.Warn("pixellab: render message", "key", key, "err", err)
		return key
	}
	return text
}

type localeRoot struct{}

func (localeRoot) Open(name string) (fs.File, error) {
	return localeFiles.Open("locales/" + name)
}
