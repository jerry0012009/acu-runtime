# Runtime 0.1.75 Existing Model Catalog Refresh

Root cause: capabilities GET read settings and library only, never refreshing
the provider's saved media catalog. Previous 2.6 acceptance created fresh
providers and explicitly refreshed them, missing existing-user stale catalogs.

Opening Images and Search now shows cached choices, then POSTs an authenticated
Runtime refresh endpoint. It verifies the selected ACU provider, delegates to
native /api/provider/models, and rereads settings. No keys extracted, model
table hardcoded, settings overwritten, tools assigned or Agent model changed.
Native source refresh may reconcile removed provider models using its normal
policy; this release does not replace that native behavior.

Refresh icon allows retry directly. Failure retains choices and a short error;
drafts survive refresh, page departure discards late responses, configuration
writes are blocked during refresh. Current 2.5 selection is not replaced by 2.6.

Acceptance:
- 125 JavaScript tests, Go/race tests and vet.
- Native Magpie v0.1.4096 integration starts with only 2.5, adds both 2.6 IDs at
  the fixture vendor, confirms cached GET remains old, refreshes through Runtime,
  then checks both 2.6 candidates and retained selected 2.5. Four-Agent native
  setup, MCP migration, tools/list and external MCP preservation still pass.
- Playwright covers automatic old-to-new refresh, explicit retry, failure
  retention, draft protection, zero settings/tool writes, light/dark 390/1440
  layouts. Screenshots: /tmp/acu-capability-refresh/output/capability-refresh.
- Regression is included in every platform's release workflow.

No Router/Console/Private restart or model permission changes. Runtime packages
only. Actual user Windows profile is not accessible from this host.
