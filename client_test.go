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
	if _, err := reader.generate(t.Context(), pathNew, payload{}, false, nil); !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer for a message over %d bytes", err, maxMessageBytes)
	}
}
