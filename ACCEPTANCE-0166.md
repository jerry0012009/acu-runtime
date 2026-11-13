# Runtime 0.1.66 Fable 5.1 Recommendations

## Root Cause And Behavior

The authenticated public model list correctly provides `claude-fable-5-1`, but
Runtime's curated recommendation list still included only `claude-fable-5`.
Recommendations, ordinary model selectors and recommended visibility share that
list, so a supported Fable 5.1 entry could be hidden after setup. This is distinct
from the noncanonical `claude-fable-5.1` request spelling fixed in the server.

Fable 5.1 replaces Fable 5 in the curated list. Account availability and Agent
options still determine which models can be offered. `acu-auto` stays the general
default, explicit selected legacy models remain available, and unrelated provider
entries are unchanged. WorkBuddy continues to select models inside its own task.

Repeated batch setup for an already-connected WorkBuddy now reveals current
recommendations without hiding existing entries or replacing its selected model.
Other Agents retain their previous preserve-model/visibility behavior. Initial
setup still hides nonrecommended ACU entries unless selected/in use. Updates do
not silently write Agent configuration: users confirm one-click configuration or
the individual recommendation to apply the list.

## Verification

- Reproduced the bug first with two failing recommendation/setup regression tests.
- 122 JavaScript tests, Go race suite, Go vet and app syntax checks passed.
- Added focused coverage for exact canonical Fable 5.1 ID, unavailable models,
  ordinary selectors, protected active Fable 5, fresh WorkBuddy visibility and
  existing WorkBuddy allowlist preservation.
- Browser fixture passed fresh/existing WorkBuddy setup in light/dark at
  390/1440 widths, correct recommendation options, no overflow/script errors,
  preserved model requests and exact visibility writes. Fixture coverage is not
  a claim of configuration on the user's live desktop.
- The same browser regression is included in the three-platform release workflow.
  Native desktop, packaged kernel, old installer upgrade and installation checks
  remain enabled. Final release evidence is recorded after publication.

No Router/Console/Private service deployment, schema, environment or Key change
is required. Official website downloads resolve the latest stable Runtime release
automatically and retain the previously deployed accelerated download route.

## Published

Workflow `43504275198` passed all three platforms and published 0.1.66 from
`2a2cdf329fd5ce41b7de4ff65af3e06a14262b3f` at 2026-11-13 20:55:33 UTC.
The website resolver returns 0.1.66. Full accelerated downloads matched official
GitHub sizes and SHA-256 digests for Windows, Apple Silicon and Intel Mac.
Evidence: `/tmp/acu-runtime-speed-verification.json`.

Separately, the owner's Fable 5 requests were traced to Wawapii Kiro returning
`claude-opus-5-5` for a `claude-fable-5` request. A direct Fable 5.1 request to
that same channel also returned the Opus name. Runtime did not rewrite the model.
This recommendation release does not change upstream routing or identity policy.
