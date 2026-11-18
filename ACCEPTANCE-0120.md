# Runtime 0.1.120 Model Visibility

## Confirmed Integration Gaps

Official Magpie v0.1.4350 already provides `kept` model metadata and the `widen`
visibility operation. A model excluded by provider/family visibility remains in
the full management list with hidden=true, kept=true. Sending hidden/shown alone
does not reveal it: upstream deliberately excludes kept entries until the
source visibility is widened. Runtime's typed inventory dropped kept and its
save operation dropped widen, so enabled-looking choices could remain excluded
from the actual Codex catalog. No new catalog writer is required.

Runtime also disabled every native own-model switch and omitted own entries
from hidden payloads. Listed native GPT-6.2 entries can now be switched using
the same upstream visibility API. Enabling one while the native master switch
is off converts the other currently suppressed native entries to individual
hides before lifting the master; unused siblings remain hidden. Native options
not present in the managed catalog retain their master-only control.

## Verification

- Official SHA-verified Magpie v0.1.4350, isolated temporary profile, no account
  credentials or model inference: four extra-source Carbon/Argon/GPT-6.2 models
  are kept before widening, become shown after Runtime's native batch operation,
  and appear in the actual generated magpie-models.json. Both ordinary hidden
  lists and only-picked mode are covered; asynchronous file writes are awaited.
- A control operation without widen leaves models kept/hidden.
- Native metadata serialization, direct and batched widen forwarding, expected
  snapshot conflict checks and native operation ordering have focused tests.
- JavaScript unit regression covers both GPT-6.2 own models, the native master
  switch, sibling preservation and four kept ACU entries in both selection modes.
- Playwright inventory test passes light/dark at 390/1440 widths, actual switch
  clicks, save payload with widen, retry, cancellation and persistence. Screenshots:
  /root/jerry/exports/runtime-model-visibility-20261118/output/inventory-safe/.
- Go race suite, vet and 159 JavaScript tests passed before final release checks.
  Windows native packaging and actual installed-Codex tests run in release CI.

## Scope And Limits

Official Magpie stays v0.1.4350 and unmodified. An initially suspected upstream
desktop-cache fix is already in this version; upgrading to v0.1.4402 is not
represented as the repair. CLIProxyAPI, server supply, pricing, permissions,
default models, login mode and tokens are unchanged. Kept entries may be listed
without being usable upstream; display does not establish model availability.

Customer-device filtered state is not accessible from this host. The defect is
reproduced using synthetic inputs, not claimed as a captured customer request.
After installing, explicitly enable the desired entries, click Save and fully
reopen ChatGPT/Codex to read the generated catalog. Do not erase account/session
files, reset all visibility preferences or silently change the model provider.
