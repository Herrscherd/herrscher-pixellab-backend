package pixellab

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	maxMessageBytes = 64 << 20
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
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: connection ended before the final image: %w", ErrServer, err)
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

func interpret(raw []byte, abortOnSlow bool, progress func(int)) ([]wireImage, bool, error) {
	var message wireMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return nil, false, fmt.Errorf("%w: unreadable message: %w", ErrServer, err)
	}
	if reason := serverReason(message); reason != "" {
		return nil, false, fmt.Errorf("%w: %s", ErrServer, reason)
	}
	if message.Progress != nil && message.Image == nil && len(message.Images) == 0 {
		if message.Type == "slow" && abortOnSlow {
			return nil, false, ErrSubscriptionDry
		}
		if progress != nil {
			progress(int(math.Round(*message.Progress * percentScale)))
		}
		return nil, false, nil
	}
	images := message.Images
	if len(images) == 0 && message.Image != nil {
		images = []wireImage{*message.Image}
	}
	if len(images) > 0 && images[0].Base64 != "" {
		return images, true, nil
	}
	return nil, false, nil
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
