package pixellab

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateRefusesAMessageOverTheLimit(t *testing.T) {
	oversized := `{"type":"final","image":{"base64":"` + strings.Repeat("A", maxMessageBytes) + `"}}`
	fake := &fakePixellab{script: func(string, string) []string { return []string{oversized} }}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	reader := client{endpoint: "ws" + strings.TrimPrefix(server.URL, "http"), timeout: 30 * time.Second}
	if _, err := reader.generate(t.Context(), pathNew, payload{}, false, nil); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("error = %v, want ErrMessageTooLarge for a message over %d bytes", err, maxMessageBytes)
	}
}

func TestGenerateAcceptsTheMaxFramesBeforeTheFinalImage(t *testing.T) {
	images, err := generateAfterProgressFrames(t, maxReplyFrames)
	if err != nil {
		t.Fatalf("error = %v, want the final image after %d progress frames", err, maxReplyFrames)
	}
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}
}

func TestGenerateRefusesOneFrameOverTheLimit(t *testing.T) {
	if _, err := generateAfterProgressFrames(t, maxReplyFrames+1); !errors.Is(err, ErrTooManyFrames) {
		t.Fatalf("error = %v, want ErrTooManyFrames after %d progress frames", err, maxReplyFrames+1)
	}
}

func generateAfterProgressFrames(t *testing.T, count int) ([]wireImage, error) {
	t.Helper()
	script := make([]string, 0, count+1)
	for range count {
		script = append(script, `{"type":"tool","progress":0.5}`)
	}
	script = append(script, finalMessage(1, 1, 0))
	fake := &fakePixellab{script: func(string, string) []string { return script }}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	reader := client{endpoint: "ws" + strings.TrimPrefix(server.URL, "http"), timeout: 30 * time.Second}
	return reader.generate(t.Context(), pathNew, payload{}, false, nil)
}
