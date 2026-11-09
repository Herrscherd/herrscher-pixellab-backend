package pixellab

import (
	"os"
	"testing"

	contracts "github.com/Herrscherd/herrscher-contracts"
)

func TestLivePixellabGeneratesAnImage(t *testing.T) {
	if os.Getenv("PIXELLAB_LIVE") != "1" {
		t.Skip("set PIXELLAB_LIVE=1 to call the real PixelLab API")
	}
	backend, err := newBackend(contracts.PluginConfig{}, newMessages())
	if err != nil {
		t.Fatalf("newBackend: %v", err)
	}
	outbox := t.TempDir()
	if _, err := backend.Respond(t.Context(), contracts.Prompt{Content: "--size 16 a red heart", Outbox: outbox}, nil); err != nil {
		t.Fatalf("Respond against the live API: %v", err)
	}
	if files := backend.TakeFiles(); len(files) == 0 {
		t.Fatal("live generation delivered no file")
	}
}
