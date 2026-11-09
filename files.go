package pixellab

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var errFileTooLarge = errors.New("file is larger than the limit")

func readBounded(path string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer closeQuietly(root)
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer closeQuietly(file)
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%w: %s, %d bytes allowed", errFileTooLarge, path, limit)
	}
	return raw, nil
}

func closeQuietly(closer io.Closer) {
	if err := closer.Close(); err != nil {
		slog.Debug("pixellab: close file", "err", err)
	}
}
