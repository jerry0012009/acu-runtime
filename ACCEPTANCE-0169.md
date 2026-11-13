# Runtime 0.1.69 Codex Compatibility

## Cause And Fix

Runtime forced `codex login api` on every prefixed model change, ignoring the
native default and explicit user selections. Its app-server probe then required
a non-null account whenever auth.json contained auth_mode=chatgpt. Native API,
forced API, and exhausted-account states can legitimately return no account;
OpenAI's app-server documentation also includes account=null and
requiresOpenaiAuth=false. Historical auth.json alone is not active identity.

Model changes now retain the native login policy. The native default remains
automatic; explicit api/chatgpt settings remain untouched. Native configuration
and model-list checks still run. The probe and quiet transaction verify auth.json
is unchanged rather than interpreting a null account as identity loss. No login,
logout, token refresh, GUI launch or paid inference is initiated by the probe.
Idle-watcher pause, exact route rollback and gateway recovery remain intact.

The embedded native UI keeps original values and backend behavior, replacing
only Codex login labels/notes: automatic compatibility (recommended), ChatGPT
only, ACU API. Interned shared option lists and other agents are not modified.

## Verification

- Real installed Codex plus official SHA-verified Magpie v0.1.3940: default mode
  loads selected ACU model, all three native login modes survive model changes,
  missing model fails, route rollback/cancellation/disconnect and external MCP
  preservation pass in an isolated Linux profile. No user profile modified.
- Cross-platform subprocess regression tests: historical ChatGPT credentials and
  absent credentials both load; wrong model and credential mutation fail. A
  fixture exits if account/read is called, proving the redundant gate is gone.
- Go race suite, vet, 123 JavaScript tests pass. Native picker browser checks
  concise labels, unchanged values, light/dark and 390/1440 layouts.
- Browser evidence: /tmp/acu-codex-login-browser/output/codex-login.
- Windows release checks retain the actual installed Codex app-server transaction,
  three native login choices, failure rollback and installer upgrade tests.

Reference: https://developers.openai.com/codex/app-server/

Only Runtime packages are released; no Router/Console/Private restart or schema,
environment, existing API Key or user authentication configuration change.
