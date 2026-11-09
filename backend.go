package pixellab

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	contracts "github.com/Herrscherd/herrscher-contracts"
	"github.com/Herrscherd/herrscher-contracts/i18n"
)

const (
	progressKind     = "tool"
	progressTool     = "pixellab"
	outputPattern    = "pixellab-%02d.png"
	outputType       = "image/png"
	outputPermission = 0o600
	redacted         = "[redacted]"
)

// Backend answers a herrscher prompt by generating pixel art with PixelLab.
type Backend struct {
	settings settings
	accounts []account
	client   client
	texts    *messages
	turn     sync.Mutex
	filesMu  sync.Mutex
	files    []contracts.OutboundFile
}

func newBackend(cfg contracts.PluginConfig, texts *messages) (*Backend, error) {
	config, err := parseSettings(cfg)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	accounts, err := loadAccounts(config, home)
	if err != nil {
		return nil, err
	}
	return &Backend{
		settings: config,
		accounts: accounts,
		client:   client{endpoint: config.endpoint, timeout: config.timeout},
		texts:    texts,
	}, nil
}

// Respond generates the images a prompt asks for and writes them to its outbox.
func (b *Backend) Respond(ctx context.Context, prompt contracts.Prompt, onEvent func(contracts.BackendEvent)) (string, error) {
	b.turn.Lock()
	defer b.turn.Unlock()
	if prompt.Outbox == "" {
		return "", ErrNoOutbox
	}
	opts, err := parseOptions(prompt.Content, b.settings)
	if err != nil {
		return "", err
	}
	request, err := prepareJob(opts, prompt.Attachments)
	if err != nil {
		return "", err
	}
	base, err := b.baseSeed(opts)
	if err != nil {
		return "", err
	}
	lang := i18n.Resolve(prompt.Language)
	progress := b.progressSink(onEvent, lang)
	state := newFailover()
	var produced []contracts.OutboundFile
	for index := range request.count {
		images, err := b.generateWithFailover(ctx, request, base+int64(index), state, progress)
		if err != nil {
			b.storeFiles(produced)
			return "", b.redact(err)
		}
		for _, image := range images {
			file, err := b.writeImage(prompt.Outbox, len(produced)+1, image)
			if err != nil {
				b.storeFiles(produced)
				return "", err
			}
			produced = append(produced, file)
		}
	}
	b.storeFiles(produced)
	return b.texts.render(lang, replyDelivered, map[string]any{"Count": len(produced)}), nil
}

// TakeFiles returns the files produced since the last call and clears them.
func (b *Backend) TakeFiles() []contracts.OutboundFile {
	b.filesMu.Lock()
	defer b.filesMu.Unlock()
	files := b.files
	b.files = nil
	return files
}

// Close releases nothing: each request opens and closes its own connection.
func (b *Backend) Close() error {
	return nil
}

func (b *Backend) baseSeed(opts options) (int64, error) {
	if opts.seedGiven {
		return opts.seed, nil
	}
	return randomSeed()
}

func (b *Backend) progressSink(onEvent func(contracts.BackendEvent), lang i18n.Language) func(int) {
	return func(percent int) {
		if onEvent == nil {
			return
		}
		onEvent(contracts.BackendEvent{
			Kind:   progressKind,
			Tool:   progressTool,
			Detail: b.texts.render(lang, progressPercent, map[string]any{"Percent": percent}),
		})
	}
}

func (b *Backend) generateWithFailover(ctx context.Context, request job, seed int64, state *failover, progress func(int)) ([]wireImage, error) {
	abortOnSlow := len(b.accounts) > 1
	for index, acc := range b.accounts {
		if state.skipped(acc.secret) {
			continue
		}
		images, err := b.client.generate(ctx, request.path(), request.payload(acc, seed), abortOnSlow, progress)
		if err == nil {
			return images, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrSubscriptionDry) {
			state.dry[acc.secret] = true
			slog.Info("pixellab: account out of subscription, switching", "account", index+1)
			continue
		}
		state.dead[acc.secret] = true
		slog.Warn("pixellab: account failed, switching", "account", index+1, "err", b.redact(err).Error())
	}
	alive := state.alive(b.accounts)
	if len(alive) == 0 {
		return nil, ErrNoDelivery
	}
	slog.Info("pixellab: every account is dry, riding the slow queue")
	return b.client.generate(ctx, request.path(), request.payload(alive[0], seed), false, progress)
}

func (b *Backend) writeImage(outbox string, number int, image wireImage) (contracts.OutboundFile, error) {
	raw, size, err := decodeFrame(image.Base64, imageSize{Width: image.Width, Height: image.Height})
	if err != nil {
		return contracts.OutboundFile{}, err
	}
	data, err := encodePNG(raw, size.Width, size.Height)
	if err != nil {
		return contracts.OutboundFile{}, err
	}
	name := fmt.Sprintf(outputPattern, number)
	path := filepath.Join(outbox, name)
	if err := os.WriteFile(path, data, outputPermission); err != nil {
		return contracts.OutboundFile{}, fmt.Errorf("write %s: %w", name, err)
	}
	file := contracts.OutboundFile{Path: path, Name: name, ContentType: outputType}
	if err := contracts.ValidateOutboundFile(file, outbox); err != nil {
		return contracts.OutboundFile{}, err
	}
	return file, nil
}

func (b *Backend) storeFiles(files []contracts.OutboundFile) {
	b.filesMu.Lock()
	defer b.filesMu.Unlock()
	b.files = append(b.files, files...)
}

func (b *Backend) redact(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	for _, acc := range b.accounts {
		message = strings.ReplaceAll(message, acc.secret, redacted)
	}
	if message == err.Error() {
		return err
	}
	return redactedError{message: message, kept: keptSentinels(err)}
}

type redactedError struct {
	message string
	kept    []error
}

func (e redactedError) Error() string {
	return e.message
}

func (e redactedError) Unwrap() []error {
	return e.kept
}

func keptSentinels(err error) []error {
	var kept []error
	for _, sentinel := range redactionSentinels() {
		if errors.Is(err, sentinel) {
			kept = append(kept, sentinel)
		}
	}
	return kept
}

type failover struct {
	dry  map[string]bool
	dead map[string]bool
}

func newFailover() *failover {
	return &failover{dry: make(map[string]bool), dead: make(map[string]bool)}
}

func (f *failover) skipped(secret string) bool {
	return f.dry[secret] || f.dead[secret]
}

func (f *failover) alive(accounts []account) []account {
	var alive []account
	for _, acc := range accounts {
		if !f.dead[acc.secret] {
			alive = append(alive, acc)
		}
	}
	return alive
}
