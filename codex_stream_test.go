package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		n := calls.Add(1)
		if err != nil || !json.Valid(body) || !strings.Contains(string(body), "history-marker") || len(body) < 640<<10 {
			t.Errorf("turn %d: incomplete history upload (%d bytes): %v", n, len(body), err)
		}
		if strings.Contains(string(body), "previous_response_id") {
			t.Error("local continuation ID leaked to HTTP upstream")
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
	if err := kernel.exchange(ctx, "POST", "/api/provider/save", map[string]any{
		"id": "acu-runtime", "new": true, "name": "ACU fixture", "key": key,
		"responses": base, "socket": "off", "models": []string{model},
	}, nil); err != nil {
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
	turn := func(conn *websocket.Conn, text, previous string) string {
		t.Helper()
		payload := map[string]any{"type": "response.create", "model": "acu-runtime/" + model,
			"store": false, "instructions": "For this test reply with just OK.", "max_output_tokens": 32,
			"input": []map[string]string{{"role": "user", "content": text}}}
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
	conn := connect()
	defer conn.CloseNow()
	if live {
		turn(conn, "Reply OK.", "")
		conn.CloseNow()
		conn = connect()
		defer conn.CloseNow()
	}
	history := strings.Repeat("history-marker: synthetic archived notes, no actions needed.\n", 12000)
	id := turn(conn, history+"Reply OK.", "")
	turn(conn, "Reply OK again.", id)
	if !live && calls.Load() != 2 {
		t.Fatalf("expected exactly two complete HTTP uploads, got %d", calls.Load())
	}
}
