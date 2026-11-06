package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T, upstream http.Handler) (*server, *httptest.Server) {
	t.Helper()
	remote := httptest.NewServer(upstream)
	t.Cleanup(remote.Close)
	kernel, err := newKernel(remote.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &server{kernel: kernel, host: "127.0.0.1:4317", token: "test-local-session", web: http.NotFoundHandler()}, remote
}

func TestKernelURLBoundary(t *testing.T) {
	for _, address := range []string{"https://127.0.0.1:1", "http://example.com", "http://localhost:1", "http://user@127.0.0.1", "http://127.0.0.1/?k=secret", "http://127.0.0.1/api", "http://127.0.0.1/#fragment"} {
		if _, err := newKernel(address); err == nil {
			t.Errorf("accepted unsafe address %q", address)
		}
	}
	for _, address := range []string{"http://127.0.0.1:1234", "http://[::1]:1234/"} {
		if _, err := newKernel(address); err != nil {
			t.Errorf("rejected loopback origin: %v", err)
		}
	}
}

func TestOnboardingStatusProjectsConnectAndRestartFacts(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		io.WriteString(response, `{"agents":[
			{"id":"codex","wired":true,"stale":2,"staleCopies":[{"kind":"ide","path":"private-editor-path"},{"kind":"cli"},{"kind":"unexpected-secret"}],"fields":[{"key":"model","value":"acu/auto","options":[{"ref":"acu/auto","value":"acu/auto","label":"acu-auto","group":"ACU"},{"value":"chatgpt","label":"ChatGPT"}]},{"key":"api_key","options":[{"value":"private-field-secret"}]}]},
			{"id":"workbuddy","wired":true,"appRestart":{"pid":1234,"private":"not-for-summary"},"models":{"listed":2},"fields":[]},
			{"id":"claude","cliMissing":true,"drift":{"private":"not-for-summary"},"fields":[{"key":"model","options":[{"ref":"acu/auto"}]}]},
			{"id":"hermes","appRestart":null,"drift":null,"fields":[]}
		],"apiKey":"not-for-summary"}`)
	}))
	response := httptest.NewRecorder()
	host.status(response, httptest.NewRequest("GET", "http://"+host.host+"/runtime/status", nil))
	var status struct {
		Ready  bool        `json:"ready"`
		Agents []agentView `json:"agents"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Ready || len(status.Agents) != 4 {
		t.Fatalf("invalid status: %+v", status)
	}
	if !status.Agents[0].CanConnect || !status.Agents[0].RestartRequired || strings.Join(status.Agents[0].RestartKinds, ",") != "ide,cli" {
		t.Fatalf("Codex instructions missing: %+v", status.Agents[0])
	}
	if options := status.Agents[0].ModelOptions; len(options) != 2 || options[0].Value != "acu/auto" || options[0].Label != "acu-auto" || options[0].Group != "ACU" || options[1].Value != "chatgpt" {
		t.Fatalf("native model choices or existing provider choices were lost: %+v", options)
	}
	if roles := status.Agents[0].Roles; len(roles) != 1 || roles[0].Ref != "acu/auto" || roles[0].Key != "model" {
		t.Fatalf("selected role projection missing or exposed unrelated fields: %+v", roles)
	}
	if !status.Agents[2].CanConnect || !status.Agents[2].RestartRequired || strings.Join(status.Agents[2].RestartKinds, ",") != "app" {
		t.Fatalf("WorkBuddy restart missing: %+v", status.Agents[2])
	}
	if status.Agents[1].Detected || status.Agents[1].CanConnect || !status.Agents[1].NeedsRepair || status.Agents[3].RestartRequired || status.Agents[3].NeedsRepair || status.Agents[3].CanConnect {
		t.Fatalf("incorrect connection eligibility: %+v", status.Agents)
	}
	for _, private := range []string{"not-for-summary", "private-editor-path", "unexpected-secret", "1234", "private-field-secret"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("private native state leaked: %s", private)
		}
	}
}

func TestBrowserBoundary(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	for _, check := range []struct{ name, method, path, hostname, origin, site, token string }{
		{"host", "GET", "/", "evil.test", "", "", ""},
		{"origin", "POST", "/api/set", host.host, "https://evil.test", "", ""},
		{"fetch-site", "GET", "/runtime/status", host.host, "", "cross-site", ""},
		{"missing-session", "POST", "/runtime/kernel/start", host.host, "", "", ""},
		{"null-origin", "POST", "/api/set", host.host, "null", "", ""},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(check.method, "http://"+check.hostname+check.path, nil)
			request.Header.Set("Origin", check.origin)
			request.Header.Set("Sec-Fetch-Site", check.site)
			request.Header.Set("X-ACU-Token", check.token)
			response := httptest.NewRecorder()
			host.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestAuthenticatedProxy(t *testing.T) {
	t.Setenv("ACU_KERNEL_KEY", "isolated-management-key")
	seenWrite := false
	host, remote := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("k") == "isolated-management-key" {
			http.SetCookie(response, &http.Cookie{Name: "kernel-session", Value: "private-value", Path: "/", HttpOnly: true})
			http.Redirect(response, request, "/", http.StatusSeeOther)
			return
		}
		if cookie, err := request.Cookie("kernel-session"); err != nil || cookie.Value != "private-value" {
			response.WriteHeader(401)
			return
		}
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(response, "<html><head></head><body>native component</body></html>")
		case "/api/state":
			response.Header().Set("Content-Type", "application/json")
			io.WriteString(response, `{"agents":[{"id":"codex","wired":true,"path":"/test/config.toml","fields":[{"key":"model","value":"test-model"}]}],"privateKey":"not-for-summary"}`)
		case "/api/set":
			seenWrite = request.Method == "POST" && request.Header.Get("Origin") == "http://"+request.Host
			io.WriteString(response, "{}")
		default:
			http.NotFound(response, request)
		}
	}))
	if err := host.kernel.authenticate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(remote.URL, "http://127.0.0.1:") {
		t.Fatal("non-local test server")
	}
	for _, path := range []string{"/kernel/?acu-view=providers", "/runtime/status", "/api/set"} {
		method := "GET"
		if path == "/api/set" {
			method = "POST"
		}
		request := httptest.NewRequest(method, "http://"+host.host+path, strings.NewReader(`{}`))
		request.Header.Set("Origin", "http://"+host.host)
		response := httptest.NewRecorder()
		host.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Set-Cookie") != "" || strings.Contains(response.Body.String(), "private-value") {
			t.Fatal("kernel session exposed")
		}
		if strings.HasPrefix(path, "/kernel/") && !strings.Contains(response.Body.String(), "/ui/kernel-bridge.js") {
			t.Fatal("embed script missing")
		}
		if path == "/runtime/status" {
			var status struct {
				Ready  bool
				Agents []agentView
			}
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if !status.Ready || len(status.Agents) != 4 || status.Agents[0].Model != "test-model" || !status.Agents[0].Connected {
				t.Fatalf("wrong status: %+v", status)
			}
			if strings.Contains(response.Body.String(), "not-for-summary") {
				t.Fatal("raw state leaked into summary")
			}
		}
	}
	if !seenWrite {
		t.Fatal("native configuration write not forwarded")
	}
	request := httptest.NewRequest("GET", "http://"+host.host+"/kernel/?k=browser-key", nil)
	response := httptest.NewRecorder()
	host.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatal("accepted a browser-supplied kernel key")
	}
}

func TestProxyStreamsWithoutControlTimeout(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(response, "data: first\n\n")
		response.(http.Flusher).Flush()
		time.Sleep(75 * time.Millisecond)
		io.WriteString(response, "data: second\n\n")
	}))
	host.kernel.client.Timeout = 10 * time.Millisecond
	response := httptest.NewRecorder()
	host.ServeHTTP(response, httptest.NewRequest("GET", "http://"+host.host+"/api/events", nil))
	if !response.Flushed || !strings.Contains(response.Body.String(), "second") {
		t.Fatal("stream was buffered or cut off")
	}
}

func TestShellDoesNotStartKernel(t *testing.T) {
	kernel, err := newKernel("")
	if err != nil {
		t.Fatal(err)
	}
	host := &server{kernel: kernel, host: "127.0.0.1:4317", token: "shell-token"}
	for _, path := range []string{"/", "/runtime/status"} {
		response := httptest.NewRecorder()
		host.ServeHTTP(response, httptest.NewRequest("GET", "http://"+host.host+path, nil))
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
		if path == "/" && (!strings.Contains(response.Body.String(), "shell-token") || strings.Contains(response.Body.String(), "__ACU_TOKEN__")) {
			t.Fatal("shell token not populated")
		}
	}
	if kernel.process != nil || kernel.endpoint != nil {
		t.Fatal("read-only shell started a kernel")
	}
}

func TestDesktopQuitRequiresSession(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.desktop = true
	host.quitRequests = make(chan struct{}, 1)
	for _, token := range []string{"", host.token} {
		request := httptest.NewRequest("POST", "http://"+host.host+"/runtime/desktop/quit", nil)
		request.Header.Set("X-ACU-Token", token)
		response := httptest.NewRecorder()
		host.ServeHTTP(response, request)
		if token == "" && response.Code != 403 {
			t.Fatal("untrusted quit accepted")
		}
		if token != "" && response.Code != 200 {
			t.Fatal("trusted quit rejected")
		}
	}
	if len(host.quitRequests) != 1 {
		t.Fatal("wrong quit request count")
	}
}

func TestStartupIsNonBlockingAndDeduplicated(t *testing.T) {
	entered, release := make(chan struct{}, 2), make(chan struct{})
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		entered <- struct{}{}
		<-release
		response.WriteHeader(http.StatusOK)
	}))
	host.kernel.key = "isolated-key"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host.kernel.begin(ctx)
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("startup worker did not run")
	}
	host.kernel.begin(ctx)
	started := time.Now()
	response := httptest.NewRecorder()
	host.status(response, httptest.NewRequest("GET", "http://"+host.host+"/runtime/status", nil))
	close(release)
	if time.Since(started) > time.Second || !strings.Contains(response.Body.String(), `"starting":true`) {
		t.Fatal("status blocked on a starting kernel")
	}
	deadline := time.Now().Add(time.Second)
	for {
		host.kernel.mu.Lock()
		starting := host.kernel.starting
		host.kernel.mu.Unlock()
		if !starting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("startup did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if len(entered) != 0 {
		t.Fatal("duplicate startup was launched")
	}
}
