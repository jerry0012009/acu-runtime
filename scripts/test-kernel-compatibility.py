"""Exercise the packaged kernel and Runtime in a disposable user profile.

No real keys, agent launches or paid model requests. Run on release runners.
"""
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import plistlib
import re
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
import threading
import queue
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


kernel, runtime = [str(Path(p).resolve()) for p in sys.argv[1:3]]
flags = {"creationflags": subprocess.CREATE_NO_WINDOW} if os.name == "nt" else {}
class ModelList(BaseHTTPRequestHandler):
    def do_GET(self):
        payload = json.dumps({"data": [{"id": "acu-auto"}, {"id": "gpt-6.1-sol", "input_modalities": ["text", "image"]}, {"id": "gpt-image-2.5-sunburst-2026-09-08", "kind": "image"}]}).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(payload)
    def log_message(self, *args):
        pass
vendor = ThreadingHTTPServer(("127.0.0.1", 0), ModelList)
threading.Thread(target=vendor.serve_forever, daemon=True).start()
with tempfile.TemporaryDirectory(prefix="acu-kernel-compat-") as folder:
    root = Path(folder)
    env = os.environ.copy()
    for name in ("USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME",
                 "XDG_DATA_HOME", "XDG_CACHE_HOME", "HERMES_HOME", "CODEX_HOME",
                 "CLAUDE_CONFIG_DIR", "WORKBUDDY_CONFIG_DIR"):
        env[name] = str(root / name)
        Path(env[name]).mkdir()
    (Path(env["CODEX_HOME"]) / "config.toml").write_text(
        '[mcp_servers.mine]\ncommand = "existing-fixture-mcp"\nargs = []\n')
    (Path(env["HERMES_HOME"]) / "config.yaml").write_text(
        'model:\n  default: fixture\nmcp_servers:\n  mine:\n    command: existing-fixture-mcp\n')
    (Path(env["CLAUDE_CONFIG_DIR"]) / "settings.json").write_text('{}')
    # Inert vendor-install markers: a clean runner has no actual Agent CLIs.
    # New kernels correctly mark config-only leftovers as uninstalled. This
    # test exercises config APIs, not Agent execution or CLI installation.
    (Path(env["HOME"]) / ".codex/packages/standalone").mkdir(parents=True)
    (Path(env["HERMES_HOME"]) / "hermes-agent").mkdir()
    claude_local = Path(env["HOME"]) / ".claude/local"
    claude_local.mkdir(parents=True)
    (claude_local / ("claude.exe" if os.name == "nt" else "claude")).touch()
    # macOS detects WorkBuddy by its bundle. This inert marker is never launched.
    bundle = Path(env["HOME"]) / "Applications/WorkBuddy.app/Contents"
    bundle.mkdir(parents=True)
    (bundle / "Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": "com.tencent.workbuddy.mac"}))
    env["ACU_KERNEL_PATH"] = kernel
    env["MAGPIE_WEB_KEY"] = env["ACU_KERNEL_KEY"] = "isolated-compatibility-key"
    env["MAGPIE_ADDR"] = "127.0.0.1:" + str(free_port())
    origin = "http://127.0.0.1:" + str(free_port())
    rt = "http://127.0.0.1:" + str(free_port())
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}),
        urllib.request.HTTPCookieProcessor(jar))

    def request(url, body=None, token=None):
        headers = {"Content-Type": "application/json", "Origin": origin if url.startswith(origin) else rt}
        if token:
            headers["X-ACU-Token"] = token
        data = json.dumps(body).encode() if body is not None else None
        import http.client
        from urllib.parse import urlsplit
        parsed = urlsplit(url)
        probe = urllib.request.Request(url)
        jar.add_cookie_header(probe)
        if probe.has_header("Cookie"):
            headers["Cookie"] = probe.get_header("Cookie")
        connection = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=100)
        try:
            connection.request("POST" if body is not None else "GET", parsed.path + ("?" + parsed.query if parsed.query else ""), body=data, headers=headers)
            response = connection.getresponse()
            payload = response.read()
            assert response.status == 200, (url, response.status, payload[:500])
            result = json.loads(payload)
            # 0.1.3852 sends each option once, referenced by a field's
            # one-based list and the list's zero-based option indexes.
            if isinstance(result, dict) and "optionLists" in result:
                for agent in result.get("agents", []):
                    for field in agent.get("fields", []):
                        if field.get("list"):
                            field["options"] = [result["options"][i] for i in result["optionLists"][field["list"] - 1]]
            return result
        finally:
            connection.close()

    def wait(url):
        for _ in range(100):
            try:
                return client.open(url, timeout=2).read().decode()
            except OSError:
                time.sleep(0.2)
        raise AssertionError("Local service did not start: " + url)

    def config_files():
        return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
                for p in root.rglob("*") if p.is_file() and p.name in
                ("config.toml", "config.yaml", "mcp.json", ".claude.json")}

    processes = []
    with (root / "kernel.log").open("wb") as log:
        try:
            # Exercise Runtime's native update-policy commands with the new CLI.
            for args in (("update", "auto", "off", "notify"), ("settings", "set", "autoRestart", "off")):
                subprocess.run([kernel, *args], env=env, check=True, timeout=20, stdout=log, stderr=log, **flags)
            processes.append(subprocess.Popen([kernel, "web", "--addr", origin.split("//")[1], "--no-open"],
                env=env, stdout=log, stderr=log, **flags))
            wait(origin + "/?k=" + env["MAGPIE_WEB_KEY"])
            models = ["acu-auto", "gpt-6.1-sol", "gpt-image-2.5-sunburst-2026-09-08"]
            request(origin + "/api/provider/save", {"id": "acu-runtime", "name": "ACU fixture",
                "key": "not-a-real-key", "new": True, "chat": "https://acu.best/v1", "modelsURL":"http://127.0.0.1:"+str(vendor.server_port)+"/v1/models", "models": models})
            request(origin + "/api/provider/models", {"id":"acu-runtime"})
            request(origin + "/api/agents/rescan", {})
            ids = ["codex", "claude", "hermes", "workbuddy"]
            agents = {a["id"]: a for a in request(origin + "/api/state")["agents"]}
            for name in ids:
                assert name in agents and not agents[name].get("cliMissing"), name
                print("Configuring", name, flush=True)
                request(origin + "/api/agents/connect/" + name, {})
                if name != "workbuddy":
                    field = next(f for f in agents[name]["fields"] if f["key"] == "model")
                    option = next(o for o in field["options"] if o.get("ref") == "acu-runtime/acu-auto")
                    request(origin + "/api/set", {"agent": name, "field": "model", "value": option["value"]})
            # Fresh user: Magpie chooses its API provider without manufacturing
            # a ChatGPT login. This is the same kernel path Runtime delegates.
            codex_config = (Path(env["CODEX_HOME"]) / "config.toml").read_text()
            assert 'model_provider = "magpie"' in codex_config, codex_config
            assert "requires_openai_auth = true" not in codex_config, codex_config
            assert not (Path(env["CODEX_HOME"]) / "auth.json").exists()
            # Explicit Magpie API choice is available and uses its native API.
            request(origin + "/api/set", {"agent": "codex", "field": "login", "value": "api"})
            processes.append(subprocess.Popen([runtime, "--web", "--listen", rt.split("//")[1], "--kernel-url", origin],
                env=env, stdout=log, stderr=log, **flags))
            html = wait(rt)
            token = re.search(r'name="acu-token" content="([^"]+)"', html)[1]
            status = request(rt + "/runtime/status")
            assert status["ready"] and len([a for a in status["agents"] if a["connected"]]) == 4
            for name, kind in (("acu-vision", "vision"), ("acu-image", "image")):
                request(origin + "/api/library/servers/save", {"old":"", "server":{
                    "name":name, "transport":"http", "url":"http://127.0.0.1:3425/mcp/magpie/"+kind,
                    "note":"ACU Runtime recommended tools v1", "agents":["codex"], "local":True,
                    "startupTimeout":73, "disabledTools":["generate_video"] if kind == "image" else []}})
            request(origin + "/api/library/servers/save", {"old":"", "server":{
                "name":"acu-free-search", "transport":"stdio", "command":"old-fixture-uvx",
                "args":["--from", "duckduckgo-mcp-server[browser]==0.7.0", "duckduckgo-mcp-server"],
                "note":"ACU Runtime recommended tools v1", "agents":["hermes"], "local":True}})
            body = {"provider": "acu-runtime", "agents": ids}
            applied = request(rt + "/runtime/capabilities", body, token)
            repeated = request(rt + "/runtime/capabilities", body, token)
            owned = [s for s in request(origin + "/api/library")["servers"]
                     if s.get("note") == "ACU Runtime recommended tools v1"]
            legacy = next(s for s in owned if s["name"] == "acu-free-search")
            assert not legacy.get("agents"), legacy
            owned = [s for s in owned if s["name"] != "acu-free-search"]
            assert {s["name"] for s in owned} == {"acu-image", "acu-vision", "acu-search"}, owned
            for tool in owned:
                assert set(tool["agents"]) == set(ids), tool
                if tool["name"] in ("acu-vision", "acu-image"):
                    assert tool["startupTimeout"] == 73 and tool["local"], tool
                if tool["name"] == "acu-image":
                    assert tool["disabledTools"] == ["generate_video"], tool
            assert repeated["vision"] == "acu-runtime/gpt-6.1-sol" or repeated["visionAuto"], repeated
            assert repeated["imageGen"] == "" and repeated["imageGenAuto"], repeated
            assert repeated["searchReady"]
            assert "mcp_servers.mine" in (Path(env["CODEX_HOME"]) / "config.toml").read_text()
            before = config_files()
            changed = request(rt + "/runtime/capabilities", {"provider": "acu-runtime", "modelsOnly": True, "vision": "off"}, token)
            assert changed["vision"] == "off" and config_files() == before
            # Native stdio definitions and ACU names must survive real agent sync.
            for tool in owned:
                assert tool["transport"] == "stdio" and tool["args"][0] == "mcp", tool
                assert Path(tool["command"]).is_file(), tool
                proc = subprocess.Popen([tool["command"], *tool["args"]], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log, text=True, env=env, **flags)
                replies = queue.Queue()
                threading.Thread(target=lambda: [replies.put(line) for line in proc.stdout], daemon=True).start()
                def rpc(message):
                    proc.stdin.write(json.dumps(message)+'\n'); proc.stdin.flush()
                    if 'id' not in message:
                        return ''
                    while True:
                        line = replies.get(timeout=30)
                        if json.loads(line).get('id') == message['id']:
                            return line
                try:
                    rpc({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"fixture","version":"1"}}})
                    rpc({"jsonrpc":"2.0","method":"notifications/initialized"})
                    data = rpc({"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}})
                finally:
                    proc.terminate(); proc.wait(timeout=10)
                for expected in {"acu-image": ["generate_image", "generation_result"],
                                 "acu-vision": ["analyze_image"], "acu-search": ["web_search", "web_fetch"]}[tool["name"]]:
                    assert expected in data, (tool["name"], data[:500])
            print(json.dumps({"runtime": status["version"], "fourAgentsConfigured": True,
                "kernelPolicyCLI": True, "repeatNoDuplicates": True, "existingMCPPreserved": True, "legacyMigration": True,
                "sharedModels": True, "modelOnlySavePreservesAgentFiles": True, "threeMCPToolLists": True,
                "freshCodexUsesAPIWithoutChatGPTAccount": True}))
        except Exception:
            time.sleep(0.5)
            log.flush()
            print((root / "kernel.log").read_text(errors="replace")[-8000:], file=sys.stderr)
            raise
        finally:
            for proc in reversed(processes):
                proc.terminate()
                try:
                    proc.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait(timeout=10)
vendor.shutdown()
