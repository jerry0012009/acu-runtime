package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Exercise the desktop wire protocol against the packaged binary, not a
// browser mock. The default upstream is an in-process HTTP/SSE fixture.
func TestCodexResponsesHistoryOverNativeGateway(t *testing.T) {
	executable := os.Getenv("ACU_RESPONSES_KERNEL")
	if executable == "" {
		t.Skip("requires the packaged Magpie binary")
	}
	executable, err := filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "CODEX_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	t.Setenv("MAGPIE_NO_ACCOUNTS", "1")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAGPIE_ADDR", listener.Addr().String())
	listener.Close()
	model, key := "gpt-6.1-sol", "fixture-key"
	var calls atomic.Int32
	uploads := make(chan []byte, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		n := calls.Add(1)
		if err != nil || !json.Valid(body) || !bytes.Contains(body, []byte("history-marker")) {
			t.Errorf("turn %d: incomplete history upload (%d bytes): %v", n, len(body), err)
		}
		if bytes.Contains(body, []byte(`"previous_response_id"`)) {
			t.Error("local continuation ID leaked to HTTP upstream")
		}
		select {
		case uploads <- body:
		default:
			t.Error("unexpected additional HTTP upload")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		response := fmt.Sprintf(`{"id":"resp_fixture_%d","object":"response","status":"completed","model":%q,"output":[{"id":"msg_fixture","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"OK","annotations":[]}]}],"usage":{"input_tokens":6,"output_tokens":1,"total_tokens":7}}`, n, model)
		fmt.Fprintf(w, "data: {\"type\":\"response.created\",\"response\":%s}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\ndata: {\"type\":\"response.completed\",\"response\":%s}\n\n", response, response)
	}))
	defer upstream.Close()
	base := upstream.URL + "/v1"
	live := os.Getenv("ACU_RESPONSES_LIVE_BASE") != ""
	if live {
		base = os.Getenv("ACU_RESPONSES_LIVE_BASE")
		if _, err := acuBase(base); err != nil {
			t.Fatal(err)
		}
		key = os.Getenv("ACU_API_KEY")
		if key == "" {
			t.Fatal("live test requires a dedicated test key")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	kernel, err := newKernel("")
	if err != nil {
		t.Fatal(err)
	}
	kernel.exe = executable
	defer kernel.shutdown()
	if err := kernel.start(ctx); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"id": "acu-runtime", "new": true, "name": "ACU fixture", "key": key,
		"responses": base, "socket": "off", "models": []string{model},
	}
	if err := kernel.exchange(ctx, "POST", "/api/provider/save", config, nil); err != nil {
		t.Fatal(err)
	}
	addr := strings.Replace(gatewayBase(), "http:", "ws:", 1) + "/backend-api/codex/responses"
	headers := http.Header{"User-Agent": {"Codex Desktop/0.169.0-alpha.11.1 (isolated transport fixture)"}}
	connect := func() *websocket.Conn {
		conn, _, err := websocket.Dial(ctx, addr, &websocket.DialOptions{HTTPHeader: headers})
		if err != nil {
			t.Fatal(err)
		}
		return conn
	}
	turn := func(conn *websocket.Conn, input []map[string]any, previous string) string {
		t.Helper()
		payload := map[string]any{"type": "response.create", "model": "acu-runtime/" + model,
			"store": false, "instructions": "For this test reply with just OK.", "max_output_tokens": 32,
			"input": input}
		if previous != "" {
			payload["previous_response_id"] = previous
		}
		encoded, _ := json.Marshal(payload)
		started := time.Now()
		if err := conn.Write(ctx, websocket.MessageText, encoded); err != nil {
			t.Fatal(err)
		}
		var output bool
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("stream ended before completion: %v", err)
			}
			var event struct {
				Type     string                `json:"type"`
				Delta    string                `json:"delta"`
				Status   int                   `json:"status"`
				Error    struct{ Code string } `json:"error"`
				Response struct{ ID string }   `json:"response"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			switch event.Type {
			case "error", "response.failed", "response.incomplete":
				t.Fatalf("stream failed: %s (status %d, code %s)", event.Type, event.Status, event.Error.Code)
			case "response.output_text.delta":
				output = output || event.Delta != ""
			case "response.completed":
				if !output || event.Response.ID == "" {
					t.Fatal("completion had no streamed answer or response ID")
				}
				t.Logf("completed desktop-shaped turn: sent=%d bytes elapsed=%s", len(encoded), time.Since(started).Round(time.Millisecond))
				return event.Response.ID
			}
		}
	}
	textTurn := func(conn *websocket.Conn, text, previous string) string {
		return turn(conn, []map[string]any{{"role": "user", "content": text}}, previous)
	}
	conn := connect()
	defer conn.CloseNow()
	if live {
		textTurn(conn, "Reply OK.", "")
		conn.CloseNow()
		conn = connect()
		defer conn.CloseNow()
	}
	history := strings.Repeat("history-marker: synthetic archived notes, no actions needed.\n", 12000)
	id := textTurn(conn, history+"Reply OK.", "")
	textTurn(conn, "Reply OK again.", id)
	if live {
		return
	}
	nextUpload := func() []byte {
		t.Helper()
		select {
		case sent := <-uploads:
			return sent
		case <-ctx.Done():
			t.Fatal("no completed HTTP upload")
			return nil
		}
	}
	for range 2 {
		if sent := nextUpload(); len(sent) < 640<<10 {
			t.Fatalf("text history did not reach the upstream: %d bytes", len(sent))
		}
	}

	// Windows incident: the bulk of an 8.9 MB replay was inline images in
	// function results. A per-model cap is opt-in; ordinary routes keep them.
	var pngData bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 375, 400))); err != nil {
		t.Fatal(err)
	}
	imagePart := map[string]string{"type": "input_image", "image_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData.Bytes())}
	textPart := func(text string) map[string]string { return map[string]string{"type": "input_text", "text": text} }
	input := []map[string]any{{"role": "user", "content": "history-marker: inspect screenshots"}}
	for i := range 11 {
		callID := fmt.Sprintf("call_%d", i)
		input = append(input, map[string]any{"type": "function_call", "call_id": callID, "name": "screenshot", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": callID, "output": []map[string]string{textPart("screenshot-result-marker"), imagePart}})
	}
	input = append(input, map[string]any{"role": "user", "content": []map[string]string{textPart("latest-message-marker"), imagePart}})
	for _, count := range []int{12, 2} {
		if count == 2 {
			config["new"] = false
			config["maxImages"] = map[string]int{model: count}
			if err := kernel.exchange(ctx, "POST", "/api/provider/save", config, nil); err != nil {
				t.Fatal(err)
			}
		}
		turn(conn, input, "")
		sent := nextUpload()
		assertImageHistoryUpload(t, sent, count)
		if count == 12 && len(sent) < 9_000_000 || count == 2 && len(sent) > 2_000_000 {
			t.Errorf("unexpected HTTP history size with %d images: %d bytes", count, len(sent))
		}
		t.Logf("HTTP history upload with %d images: %d bytes", count, len(sent))
	}
	if calls.Load() != 4 {
		t.Fatalf("expected exactly four complete HTTP uploads, got %d", calls.Load())
	}
}

func assertImageHistoryUpload(t *testing.T, body []byte, wantImages int) {
	t.Helper()
	if got := bytes.Count(body, []byte(`"type":"input_image"`)); got != wantImages {
		t.Errorf("upstream got %d images, want %d", got, wantImages)
	}
	var request struct {
		Input []struct {
			Type    string          `json:"type"`
			CallID  string          `json:"call_id"`
			Output  json.RawMessage `json:"output"`
			Content json.RawMessage `json:"content"`
		} `json:"input"`
	}
	if err := json.Unmarshal(body, &request); err != nil || len(request.Input) == 0 {
		t.Fatal("invalid native Responses history")
	}
	latest := request.Input[len(request.Input)-1].Content
	if !bytes.Contains(latest, []byte("latest-message-marker")) || !bytes.Contains(latest, []byte(`"type":"input_image"`)) {
		t.Error("the latest user message lost its text or image")
	}
	calls, outputs := map[string]bool{}, map[string]bool{}
	for _, item := range request.Input {
		switch item.Type {
		case "function_call":
			calls[item.CallID] = true
		case "function_call_output":
			outputs[item.CallID] = true
			if !bytes.Contains(item.Output, []byte("screenshot-result-marker")) {
				t.Errorf("tool output text changed for %s", item.CallID)
			}
			if wantImages == 2 && item.CallID == "call_0" {
				if bytes.Contains(item.Output, []byte(`"type":"input_image"`)) || !bytes.Contains(item.Output, []byte("An earlier image was omitted")) {
					t.Error("the oldest image was not replaced with a text placeholder")
				}
			}
		}
	}
	for i := range 11 {
		id := fmt.Sprintf("call_%d", i)
		if !calls[id] || !outputs[id] {
			t.Errorf("image policy broke the pairing for %s", id)
		}
	}
}
