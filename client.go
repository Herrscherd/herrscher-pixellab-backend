package pixellab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	maxMessageBytes = 16 << 20
	maxReplyImages  = maxGrid * maxGrid
	maxReplyFrames  = 10000
	percentScale    = 100
)

type client struct {
	endpoint string
	timeout  time.Duration
}

type wireImage struct {
	Base64 string `json:"base64"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type wireMessage struct {
	Type     string      `json:"type"`
	Error    string      `json:"error"`
	Message  string      `json:"message"`
	Progress *float64    `json:"progress"`
	Image    *wireImage  `json:"image"`
	Images   []wireImage `json:"images"`
}

func (c client) generate(ctx context.Context, path string, body payload, abortOnSlow bool, progress func(int)) ([]wireImage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, c.endpoint+path, nil)
	discardBody(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: dial %s: %w", ErrServer, path, err)
	}
	defer closeConnection(conn)
	conn.SetReadLimit(maxMessageBytes)
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %w", ErrServer, err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return nil, fmt.Errorf("%w: send request: %w", ErrServer, err)
	}
	for frames := 1; ; frames++ {
		if frames > maxReplyFrames {
			return nil, fmt.Errorf("%w: more than %d frames before the final image", ErrTooManyFrames, maxReplyFrames)
		}
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return nil, readError(err)
		}
		images, done, err := interpret(raw, abortOnSlow, progress)
		if err != nil {
			return nil, err
		}
		if done {
			return images, nil
		}
	}
}

func readError(err error) error {
	if errors.Is(err, websocket.ErrMessageTooBig) || websocket.CloseStatus(err) == websocket.StatusMessageTooBig {
		return fmt.Errorf("%w: %w", ErrMessageTooLarge, err)
	}
	return fmt.Errorf("%w: connection ended before the final image: %w", ErrServer, err)
}

func discardBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		slog.Debug("pixellab: close handshake response", "err", err)
	}
}

func closeConnection(conn *websocket.Conn) {
	if err := conn.CloseNow(); err != nil {
		slog.Debug("pixellab: close connection", "err", err)
	}
}

func decodeMessage(raw []byte) (wireMessage, bool) {
	var message wireMessage
	return message, json.Unmarshal(raw, &message) == nil
}

func interpret(raw []byte, abortOnSlow bool, progress func(int)) ([]wireImage, bool, error) {
	message, ok := decodeMessage(raw)
	if !ok {
		return nil, false, nil
	}
	if reason := serverReason(message); reason != "" {
		return nil, false, fmt.Errorf("%w: %s", ErrServer, reason)
	}
	if message.Progress != nil && message.Image == nil && len(message.Images) == 0 {
		if message.Type == "slow" && abortOnSlow {
			return nil, false, ErrSubscriptionDry
		}
		if progress != nil {
			progress(percentOf(*message.Progress))
		}
		return nil, false, nil
	}
	images := replyImages(message)
	if len(images) > maxReplyImages {
		return nil, false, fmt.Errorf("%w: %d images in one reply, at most %d expected", ErrServer, len(images), maxReplyImages)
	}
	if len(images) > 0 && allHaveData(images) {
		return images, true, nil
	}
	return nil, false, nil
}

func percentOf(fraction float64) int {
	return int(math.Round(min(max(fraction, 0), 1) * percentScale))
}

func replyImages(message wireMessage) []wireImage {
	if len(message.Images) > 0 {
		return message.Images
	}
	if message.Image != nil {
		return []wireImage{*message.Image}
	}
	return nil
}

func allHaveData(images []wireImage) bool {
	for _, image := range images {
		if image.Base64 == "" {
			return false
		}
	}
	return true
}

func serverReason(message wireMessage) string {
	switch {
	case message.Error != "":
		return message.Error
	case message.Type == "error" && message.Message != "":
		return message.Message
	case message.Type == "error":
		return "server error"
	}
	return ""
}
