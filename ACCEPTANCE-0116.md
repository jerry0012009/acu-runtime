# Runtime 0.1.116 Model Inventory Batch Save

## Cause And Ownership

Windows previously ran a complete safe catalog transaction per model toggle:
three idle samples one second apart, route snapshot, owned gateway suspension,
temporary native management process, Codex validation and gateway restoration.
Consequently each action could take 5-10 seconds. Those safety gates remain.

Runtime now keeps an in-memory draft and submits changed operations once on
Save. The existing catalogMutation wraps the whole batch, calling Magpie's
native only/shown/hidden/ownHidden/order endpoints in sequence and reading its
catalog back. Magpie's handler intentionally handles one operation per request;
we reuse it rather than inventing combined semantics or writing Agent files.
No custom kernel, new model registry, settings file writer or schema was added.

Expected catalog comparison prevents overwriting external changes. Empty and
ambiguous operations are rejected. Windows failures still use the existing
snapshot rollback. Other platforms retain the native mutation behavior; this
change does not claim a new cross-platform rollback implementation.

## Verification

- Frontend tests cover zero writes during editing, one batch submission,
  toggling back to the saved value, ordering without visibility rewrites,
  polling preservation, failure/retry, discard and duplicate-save prevention.
- Browser checks at 390/1440 widths and light/dark themes cover four edits in
  one submission, failed save retaining draft, retry, dismissal during pending
  save, explicit discard and no legacy unsafe endpoint. Screenshots are under
  /tmp/acu-inventory-batch/output/inventory-safe.
- Official packaged Magpie native-management test performs a batch through
  Runtime and reads only mode, ownHidden, selected visibility and ordering from
  the native kernel. It restores the original list before checking generated
  WorkBuddy configuration. No inference or customer file is involved.
- Windows release CI additionally exercises four catalog operations through
  one owned-kernel safe transaction and verifies native readback.

Editing no longer waits for an HTTP write per action. Saving still waits for
the original safety transaction; no fixed latency improvement is asserted for
the customer's hardware. Platform build/publication status is recorded only
after the release workflow completes.

## Published Acceptance

Release v0.1.116, application SHA 30a4c0f, published 2026-11-17T13:43:20Z.
Workflow 44090418058 completed all three platform builds and publication.
Windows first attempt timed out in the existing third native login fixture
waiting for quick-key; the same source passed its full Windows rerun without
skipping or weakening tests. Both Mac jobs passed on their first attempt.
Windows logged four native Codex catalog operations in one safe transaction
at 5.561 seconds, plus successful installation, upgrade and kernel compatibility.
This CI measurement is not a latency guarantee for customer hardware.

The public acu.best/api/acu-runtime/release endpoint returned 0.1.116 and all
three platform download URLs. Windows Setup, both Mac DMGs and app ZIPs,
installation notes and SHA256SUMS are present in the published release.
Local verification also passed 148 Node tests, Go tests/race/vet, the official
kernel management integration, Windows cross-compilation and browser checks.
