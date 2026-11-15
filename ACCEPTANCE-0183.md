# Preference Startup Preparation

Login restoration triggers read-only preparation of profile, records, activity,
controls, settings and team snapshots on the home page. The home page does not
wait for these reads, and request completion does not rerender unrelated views.
Results render progressively if the user opens preferences during preparation.
Account-generation checks and AbortController reject old-account replies.

No persistent preference cache, kernel credentials, LLM call, memory upload or
settings write is added. Logged-out startup makes no preference request. Status
polling does not duplicate requests. Existing cache and explicit refresh remain.
503 login renewal retries are bounded; background generating profiles get at
most three additional checks. Foreground graph polling remains available.

Focused tests cover restoration, duplicate suppression, logout, account switching
with late responses, renewal/revocation and bounded graph polling. Existing
browser tests assert all six preference endpoints are read before navigation,
then verify light/dark layouts, controls, source access and team isolation.
Native Windows and macOS CI remain release gates; local browser acceptance does
not claim control of an owner's device.
