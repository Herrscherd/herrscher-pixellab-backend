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

func TestGenerateRefusesMoreFramesThanTheLimit(t *testing.T) {
	frames := make([]string, maxReplyFrames+1)
	for index := range frames {
		frames[index] = `{"type":"tool","progress":0.5}`
	}
	fake := &fakePixellab{script: func(string, string) []string { return frames }}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	reader := client{endpoint: "ws" + strings.TrimPrefix(server.URL, "http"), timeout: 30 * time.Second}
	if _, err := reader.generate(t.Context(), pathNew, payload{}, false, nil); !errors.Is(err, ErrTooManyFrames) {
		t.Fatalf("error = %v, want ErrTooManyFrames after %d frames", err, maxReplyFrames)
	}
}
