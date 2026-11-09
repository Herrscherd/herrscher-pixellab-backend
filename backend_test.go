package pixellab

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	contracts "github.com/Herrscherd/herrscher-contracts"
	"github.com/coder/websocket"
)

type fakeCall struct {
	path   string
	secret string
	body   map[string]any
}

type fakePixellab struct {
	mu     sync.Mutex
	calls  []fakeCall
	script func(path, secret string) []string
}

func (f *fakePixellab) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer func() {
			if closeErr := conn.CloseNow(); closeErr != nil {
				t.Log(closeErr)
			}
		}()
		_, raw, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request is not JSON: %v", err)
			return
		}
		secret := fmt.Sprint(body["secret"])
		f.mu.Lock()
		f.calls = append(f.calls, fakeCall{path: r.URL.Path, secret: secret, body: body})
		f.mu.Unlock()
		for _, message := range f.script(r.URL.Path, secret) {
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(message)); err != nil {
				return
			}
		}
	}
}

func (f *fakePixellab) recorded() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func startFake(t *testing.T, script func(path, secret string) []string) (*Backend, *fakePixellab, func(accounts ...account) *Backend) {
	t.Helper()
	fake := &fakePixellab{script: script}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	build := func(accounts ...account) *Backend {
		s, err := parseSettings(contracts.PluginConfig{})
		if err != nil {
			t.Fatal(err)
		}
		return &Backend{
			settings: s,
			accounts: accounts,
			client:   client{endpoint: endpoint, timeout: 5 * time.Second},
			texts:    newMessages(),
			turn:     make(chan struct{}, 1),
		}
	}
	return build(account{secret: "secret-one", version: "0.4.120", tier: 1}), fake, build
}

func finalMessage(width, height int, fill byte) string {
	pixels := make([]byte, width*height*4)
	for index := range pixels {
		pixels[index] = fill
	}
	return fmt.Sprintf(`{"type":"final","image":{"type":"rgba_bytes","base64":%q,"width":%d,"height":%d}}`, base64.StdEncoding.EncodeToString(pixels), width, height)
}

func slowMessage() string {
	return `{"type":"slow","progress":0.1,"queue_position":3}`
}

func errorMessage(reason string) string {
	return fmt.Sprintf(`{"error":%q}`, reason)
}

func respondWith(t *testing.T, backend *Backend, content string, attachments []string) (string, error) {
	t.Helper()
	return backend.Respond(t.Context(), contracts.Prompt{Content: content, Attachments: attachments, Outbox: t.TempDir()}, nil)
}

func TestRespondWritesThePNGAndReplies(t *testing.T) {
	backend, _, _ := startFake(t, func(string, string) []string {
		return []string{`{"type":"tool","progress":0.5}`, finalMessage(2, 2, 255)}
	})
	var events []contracts.BackendEvent
	outbox := t.TempDir()
	reply, err := backend.Respond(t.Context(), contracts.Prompt{Content: "--size 2 red square", Outbox: outbox}, func(event contracts.BackendEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "1 image ready" {
		t.Fatalf("reply = %q, want %q", reply, "1 image ready")
	}
	files := backend.TakeFiles()
	if len(files) != 1 || files[0].Name != "pixellab-01.png" || files[0].ContentType != "image/png" {
		t.Fatalf("files = %+v, want one pixellab-01.png", files)
	}
	if again := backend.TakeFiles(); len(again) != 0 {
		t.Fatalf("second TakeFiles = %+v, want none", again)
	}
	if len(events) != 1 || events[0].Kind != progressKind || events[0].Detail != "50% generated" {
		t.Fatalf("events = %+v, want one progress line at 50%%", events)
	}
	data, err := os.ReadFile(files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := decoded.At(1, 1).RGBA(); r>>8 != 255 {
		t.Fatalf("pixel = %v, want the filled colour", decoded.At(1, 1))
	}
}

func TestRespondSendsOneSeedPerImage(t *testing.T) {
	backend, fake, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	if _, err := respondWith(t, backend, "--size 2 --n 2 --seed 10 tree", nil); err != nil {
		t.Fatal(err)
	}
	calls := fake.recorded()
	if len(calls) != 2 || calls[0].body["seed"] != float64(10) || calls[1].body["seed"] != float64(11) {
		t.Fatalf("calls = %+v, want seeds 10 and 11", calls)
	}
	if calls[0].path != pathNew || calls[0].body["model_name"] != modelNew {
		t.Fatalf("path %s model %v, want the new image endpoint", calls[0].path, calls[0].body["model_name"])
	}
}

func TestSlowAccountSwitchesAndStaysOut(t *testing.T) {
	first := account{secret: "account-a", version: "0.4.120", tier: 1}
	second := account{secret: "account-b", version: "0.4.120", tier: 1}
	_, fake, build := startFake(t, func(_, secret string) []string {
		if secret == "account-a" {
			return []string{slowMessage()}
		}
		return []string{finalMessage(2, 2, 1)}
	})
	backend := build(first, second)
	if _, err := respondWith(t, backend, "--size 2 --n 2 tree", nil); err != nil {
		t.Fatal(err)
	}
	var secrets []string
	for _, call := range fake.recorded() {
		secrets = append(secrets, call.secret)
	}
	if strings.Join(secrets, ",") != "account-a,account-b,account-b" {
		t.Fatalf("accounts used = %v, want account-a once then account-b for both images", secrets)
	}
}

func TestFailingAccountIsSkippedForTheTurn(t *testing.T) {
	first := account{secret: "account-a", version: "0.4.120", tier: 1}
	second := account{secret: "account-b", version: "0.4.120", tier: 1}
	_, fake, build := startFake(t, func(_, secret string) []string {
		if secret == "account-a" {
			return []string{errorMessage("unauthorized")}
		}
		return []string{finalMessage(2, 2, 1)}
	})
	backend := build(first, second)
	if _, err := respondWith(t, backend, "--size 2 --n 2 tree", nil); err != nil {
		t.Fatal(err)
	}
	if calls := fake.recorded(); len(calls) != 3 {
		t.Fatalf("calls = %d, want 3 (one failure, then two successes)", len(calls))
	}
}

func TestSingleAccountRidesTheSlowQueue(t *testing.T) {
	backend, _, _ := startFake(t, func(string, string) []string {
		return []string{slowMessage(), finalMessage(2, 2, 1)}
	})
	reply, err := respondWith(t, backend, "--size 2 tree", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "1 image ready" {
		t.Fatalf("reply = %q", reply)
	}
}

func TestSlowQueueRideRedactsTheSecret(t *testing.T) {
	const secret = "account-a-secret"
	var accountACalls atomic.Int32
	_, _, build := startFake(t, func(_, sent string) []string {
		if sent == "account-b-secret" {
			return []string{errorMessage("unauthorized")}
		}
		if accountACalls.Add(1) == 1 {
			return []string{slowMessage()}
		}
		return []string{errorMessage("refused " + secret)}
	})
	backend := build(
		account{secret: secret, version: "0.4.120", tier: 1},
		account{secret: "account-b-secret", version: "0.4.120", tier: 1},
	)
	_, err := respondWith(t, backend, "--size 2 tree", nil)
	if !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer from the slow queue ride", err)
	}
	for _, node := range errorTree(err) {
		if strings.Contains(node.Error(), secret) {
			t.Fatalf("error chain leaks the secret at %q", node.Error())
		}
	}
	if !strings.Contains(err.Error(), redacted) {
		t.Fatalf("error = %v, want the secret replaced by %s", err, redacted)
	}
}

func TestNoAccountDeliveringReturnsNoDelivery(t *testing.T) {
	_, _, build := startFake(t, func(_, _ string) []string {
		return []string{errorMessage("unauthorized")}
	})
	backend := build(
		account{secret: "account-a-secret", version: "0.4.120", tier: 1},
		account{secret: "account-b-secret", version: "0.4.120", tier: 1},
	)
	if _, err := respondWith(t, backend, "--size 2 tree", nil); !errors.Is(err, ErrNoDelivery) {
		t.Fatalf("error = %v, want ErrNoDelivery", err)
	}
}

func TestEditModeSendsTheAttachment(t *testing.T) {
	path := writePNG(t, 2, 2)
	backend, fake, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	if _, err := respondWith(t, backend, "--mode edit make it blue", []string{path}); err != nil {
		t.Fatal(err)
	}
	calls := fake.recorded()
	if len(calls) != 1 || calls[0].path != pathEdit {
		t.Fatalf("calls = %+v, want one edit request", calls)
	}
	field, ok := calls[0].body["edit_image"].(map[string]any)
	if !ok || field["width"] != float64(2) || calls[0].body["seed"] == nil {
		t.Fatalf("body = %+v, want an edit_image of 2 px and a seed", calls[0].body)
	}
}

func TestNewModeRejectsAttachments(t *testing.T) {
	path := writePNG(t, 2, 2)
	backend, fake, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	_, err := respondWith(t, backend, "--size 2 tree", []string{path})
	if !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("error = %v, want ErrInvalidAttachment", err)
	}
	if len(fake.recorded()) != 0 {
		t.Fatal("a rejected prompt still reached PixelLab")
	}
}

func TestAttachmentSizeMustMatchTheRequest(t *testing.T) {
	path := writePNG(t, 2, 2)
	backend, _, _ := startFake(t, func(string, string) []string { return []string{finalMessage(4, 4, 1)} })
	_, err := respondWith(t, backend, "--mode edit --size 4 make it blue", []string{path})
	if !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("error = %v, want ErrInvalidAttachment", err)
	}
}

func TestRespondNeedsAnOutbox(t *testing.T) {
	backend, _, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	_, err := backend.Respond(t.Context(), contracts.Prompt{Content: "--size 2 tree"}, nil)
	if !errors.Is(err, ErrNoOutbox) {
		t.Fatalf("error = %v, want ErrNoOutbox", err)
	}
}

func TestRespondWaitsForItsTurnOrItsContext(t *testing.T) {
	backend, fake, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	backend.turn <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := backend.Respond(ctx, contracts.Prompt{Content: "--size 2 tree", Outbox: t.TempDir()}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls := fake.recorded(); len(calls) != 0 {
		t.Fatalf("calls = %+v, want none while the turn is taken", calls)
	}
}

func TestTurnTimeoutBoundsTheWholeTurn(t *testing.T) {
	release := make(chan struct{})
	backend, _, _ := startFake(t, func(string, string) []string {
		<-release
		return []string{finalMessage(2, 2, 1)}
	})
	t.Cleanup(func() { close(release) })
	backend.settings.turnTimeout = 50 * time.Millisecond
	_, err := respondWith(t, backend, "--size 2 tree", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestInvalidOutboxWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	backend, _, _ := startFake(t, func(string, string) []string { return []string{finalMessage(2, 2, 1)} })
	_, err := backend.Respond(t.Context(), contracts.Prompt{Content: "--size 2 tree", Outbox: "relative-outbox"}, nil)
	if !errors.Is(err, contracts.ErrInvalidOutboundFile) {
		t.Fatalf("error = %v, want contracts.ErrInvalidOutboundFile", err)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %v, want none", entries)
	}
}

func TestRedactHidesSecrets(t *testing.T) {
	backend := &Backend{accounts: []account{{secret: "hidden-value"}}}
	err := backend.redact(errors.New("refused hidden-value today"))
	if strings.Contains(err.Error(), "hidden-value") {
		t.Fatalf("redacted error still shows the secret: %v", err)
	}
	if backend.redact(nil) != nil {
		t.Fatal("redact(nil) is not nil")
	}
}

func TestRedactKeepsTheSentinelAndDropsTheCause(t *testing.T) {
	backend := &Backend{accounts: []account{{secret: "hidden-value"}}}
	redactedErr := backend.redact(fmt.Errorf("%w: refused hidden-value: %w", ErrServer, errors.New("hidden-value again")))
	if !errors.Is(redactedErr, ErrServer) {
		t.Fatalf("redacted error lost ErrServer: %v", redactedErr)
	}
	for _, node := range errorTree(redactedErr) {
		if strings.Contains(node.Error(), "hidden-value") {
			t.Fatalf("a link of the chain still shows the secret: %q", node.Error())
		}
	}
}

func errorTree(err error) []error {
	if err == nil {
		return nil
	}
	nodes := []error{err}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, inner := range many.Unwrap() {
			nodes = append(nodes, errorTree(inner)...)
		}
		return nodes
	}
	if inner := errors.Unwrap(err); inner != nil {
		nodes = append(nodes, errorTree(inner)...)
	}
	return nodes
}

func TestInterpretReadsEachMessageKind(t *testing.T) {
	var progressed []int
	record := func(percent int) { progressed = append(progressed, percent) }
	if _, done, err := interpret([]byte(`{"type":"tool","progress":0.25}`), false, record); done || err != nil {
		t.Fatalf("progress: done %v err %v", done, err)
	}
	if _, _, err := interpret([]byte(slowMessage()), true, record); !errors.Is(err, ErrSubscriptionDry) {
		t.Fatalf("slow with abort: err %v, want ErrSubscriptionDry", err)
	}
	if _, _, err := interpret([]byte(errorMessage("nope")), false, record); !errors.Is(err, ErrServer) {
		t.Fatalf("error: err %v, want ErrServer", err)
	}
	images, done, err := interpret([]byte(`{"images":[{"base64":"AAAA","width":1,"height":1},{"base64":"AAAA"}]}`), false, record)
	if err != nil || !done || len(images) != 2 {
		t.Fatalf("animate final = %d images, done %v, err %v", len(images), done, err)
	}
	if _, done, err := interpret([]byte("not json"), false, record); done || err != nil {
		t.Fatalf("garbled frame: done %v, err %v, want ignored", done, err)
	}
	if len(progressed) != 1 || progressed[0] != 25 {
		t.Fatalf("progress = %v, want [25]", progressed)
	}
}

func TestInterpretWaitsUntilEveryFrameHasData(t *testing.T) {
	if _, done, err := interpret([]byte(`{"images":[{"base64":"AAAA"},{"base64":""}]}`), false, nil); done || err != nil {
		t.Fatalf("partly empty reply: done %v, err %v, want to keep waiting", done, err)
	}
}

func TestInterpretRefusesTooManyImages(t *testing.T) {
	frames := strings.TrimSuffix(strings.Repeat(`{"base64":"AAAA"},`, maxReplyImages+1), ",")
	if _, _, err := interpret([]byte(`{"images":[`+frames+`]}`), false, nil); !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer for %d images", err, maxReplyImages+1)
	}
}

func writePNG(t *testing.T, width, height int) string {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.Set(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "attachment.png")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
