# 0.1.55 Windows candidate — ready for user desktop validation

This is a candidate, not a published release. Codex integration remains disabled on the user's machine. Wait for the requested real desktop usability feedback before tagging/publishing a crash-fix release.

## Reproduced failure

The earlier candidate caused a signed-in Windows desktop failure after activation at 00:29 Asia/Shanghai on 2026-11-13. Desktop logs record a clean expected app-server exit at 16:29:24Z followed immediately by Hosted AppServerManager identity is not ready and fatal_error_broadcasted. The main ChatGPT PID stayed alive, so PID stability is not acceptance. Upstream issue #4550 documents the hot-restart failure; the native autoRestart switch does not cover Connect/Disconnect route changes.

A second configuration issue was reproduced with the installed Codex executable: beside-the-sign-in mode could parse its prefixed ACU selection but did not include it in model/list. Initialization success alone was insufficient.

## Repair

Windows Codex changes use the native Magpie API login setting for prefixed ACU models, including requires_openai_auth when the account is retained. They are applied while only Runtime's own verified idle gateway is stopped. The restarted watcher begins on the complete new route. No ChatGPT process is terminated or launched by configuration.

The actual installed Codex app-server validates initialization, account/read, config/read and presence of the selected model before committing. A ChatGPT-authenticated profile must keep an account. Failure/cancellation restores config, native catalogs, stash and applied records, and recovers the gateway. Headless config/probe descendants are contained in a Windows job and cleaned without touching existing desktop processes. The same transaction handles advanced model changes and disconnect; configuration supports auxiliary fields.

## Evidence

Official pinned Magpie **0.1.3935**, including fixed official asset digests. TestQuietCodexNativeIsolation passed using both API-only and an isolated copy of the user's ChatGPT authentication. It verified retained identity and actual ACU model visibility, workbench transaction behavior through the advanced HTTP proxy, gateway replacement, invalid-model rollback, cancellation before and after file writes, disconnect, original MCP preservation and descendant cleanup. No desktop window or inference request was started. Authentication contents are never in test output or release artifacts.

Also passed: normal and desktop-tag Go suites, vet, 114 JS tests, billing/preferences browser acceptance, six-hour refresh regression, and starfish themes/interactions/192 axes. Existing Windows WebView2 screenshots cover ordinary and bipolar fixtures; a separate actual-account screenshot is available locally. macOS native visual acceptance remains separate from browser checks.

Remaining: the user performs their requested real desktop enable/reopen/use test. No assertion of zero future crashes or completed real-desktop acceptance is made from isolated tests.

Opt-in isolated regression (Windows): ACU_QUIET_CODEX_SMOKE=1, ACU_QUIET_KERNEL=absolute official CLI path; optionally ACU_QUIET_CHATGPT_AUTH=absolute auth.json path for the signed-in case. Run go test -run ^TestQuietCodexNativeIsolation$ -v -timeout 4m . . The auth source is only copied into a disposable profile; the user's live profile stays disconnected.
