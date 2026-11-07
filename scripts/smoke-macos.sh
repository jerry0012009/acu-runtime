#!/bin/bash
set -euo pipefail
app="$1"
version="$2"
port=43871
"$app/Contents/MacOS/ACU Runtime" --listen "127.0.0.1:$port" > /tmp/acu-smoke.log 2>&1 &
runtime_pid=$!
trap 'kill "$runtime_pid" 2>/dev/null || true' EXIT
python3 - "$port" "$version" <<'PY'
import json, sys, time, urllib.request
origin = 'http://127.0.0.1:' + sys.argv[1]
for attempt in range(40):
    try:
        with urllib.request.urlopen(origin + '/runtime/status', timeout=2) as response:
            status = json.load(response)
        assert status['version'] == sys.argv[2], status
        assert status['desktop'] is True and status['platform'] == 'darwin', status
        with urllib.request.urlopen(origin + '/', timeout=2) as response:
            assert b'__ACU_TOKEN__' not in response.read()
        print('Native macOS desktop startup and local UI passed:', status['version'])
        break
    except (OSError, ValueError):
        time.sleep(0.5)
else:
    raise SystemExit('Native desktop did not start')
PY
kill -0 "$runtime_pid"
