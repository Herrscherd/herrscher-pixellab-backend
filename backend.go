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
	turn     chan struct{}
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
		turn:     make(chan struct{}, 1),
	}, nil
}

// Respond generates the images a prompt asks for and writes them to its outbox.
func (b *Backend) Respond(ctx context.Context, prompt contracts.Prompt, onEvent func(contracts.BackendEvent)) (string, error) {
	select {
	case b.turn <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-b.turn }()
	ctx, cancel := context.WithTimeout(ctx, b.settings.turnTimeout)
	defer cancel()
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

// TakeFiles returns the files produced since the last call and clears them. The
// host calls it right after each Respond, in the same turn and before any other
// Respond: the files of a Respond not taken by then are handed over with the files
// of the next turn.
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
	last := -1
	return func(percent int) {
		if onEvent == nil || percent == last {
			return
		}
		last = percent
		onEvent(contracts.BackendEvent{
			Kind:   progressKind,
			Tool:   progressTool,
			Detail: b.texts.render(lang, progressPercent, map[string]any{"Percent": percent}),
		})
	}
}

func (b *Backend) generateWithFailover(ctx context.Context, request job, seed int64, state *failover, progress func(int)) ([]wireImage, error) {
	abortOnSlow := len(b.accounts) > 1
	var last error
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
		last = err
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
		return nil, noDelivery(last)
	}
	slog.Info("pixellab: every account is dry, riding the slow queue")
	return b.client.generate(ctx, request.path(), request.payload(alive[0], seed), false, progress)
}

func (b *Backend) writeImage(outbox string, number int, image wireImage) (contracts.OutboundFile, error) {
	data, err := frameToPNG(image.Base64, imageSize{Width: image.Width, Height: image.Height})
	if err != nil {
		return contracts.OutboundFile{}, err
	}
	name := fmt.Sprintf(outputPattern, number)
	file := contracts.OutboundFile{Path: filepath.Join(outbox, name), Name: name, ContentType: outputType}
	if err := contracts.ValidateOutboundFile(file, outbox); err != nil {
		return contracts.OutboundFile{}, err
	}
	if err := createExclusive(outbox, name, data); err != nil {
		return contracts.OutboundFile{}, err
	}
	return file, nil
}

func createExclusive(dir, name string, data []byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open outbox: %w", err)
	}
	defer closeQuietly(root)
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, outputPermission)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, err := file.Write(data); err != nil {
		closeQuietly(file)
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	return nil
}

func noDelivery(last error) error {
	if last == nil {
		return ErrNoDelivery
	}
	return fmt.Errorf("%w: last failure: %w", ErrNoDelivery, last)
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
