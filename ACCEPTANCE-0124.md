# Codex Preserved-Configuration Repair

The reported one-click failure has a confirmed Runtime control-flow defect:
preserve mode returned a wired Agent's unchanged state even when native drift
was present. Windows configuration verified wiring and selected fields without
rejecting unresolved drift, leaving the UI to emit a generic failure. Native
`bypassed` (old app still on previous configuration) was also rendered as broken
configuration rather than a restart notice.

Runtime 0.1.124 reads native drift and uses official Magpie reapply for unwired
Codex under preserve mode. The retained available ACU model or already-confirmed
setup model must read back correctly and the native wiring error must disappear.
Windows performs reapply inside the existing
paused owned-gateway transaction, with snapshot rollback, restored-gateway Codex
validation and auth preservation. The advanced reapply endpoint uses the same
guard. If the existing route cannot be repaired, the same operation applies the
already-confirmed one-click model and verifies it, without a manual repair action.
An available current ACU model is preferred over replacing it. Historical old
models are not restored. Existing healthy preserved Agents retain their behavior.

Native `bypassed` is a restart-required notice, not proof of damaged files or
successful inference. The UI distinguishes it without suppressing real wiring
errors. No raw native drift details or private paths are added to status output.
Homepage example chips collapse exact release IDs across sources; full catalog
entries and provider-specific selections remain intact. No API aliases change.

Verification:

- Pre-fix regressions demonstrated preserve returning unresolved drift without
  repair, and verification accepting unresolved wiring. Regressions now pass.
- Go test/vet and 170 Node tests cover repair, failed repair, automatic recovery
  from an externally replaced model, restart-only state and summary deduplication.
- Official Magpie v0.1.4350 plus installed Codex isolation test:
  temporary model provider URL corrupted and repaired through native reapply,
  current model/MCP retained, owned watcher pause/restart, configuration readback,
  externally replaced default restored to the confirmed one-click model,
  failure rollback, canceled requests and disconnect. No live user config or
  inference was used. Windows runs the same native regression in release CI.
- Playwright recommendations cover four Agents, fresh/preserved setup, light/dark
  and 390/1440 widths, with preserved Codex drift recovery and no page overflow.

This is an application fix based on the reproducible path, not a claim that the
operator's unseen local drift cause or private configuration was inspected.
Users should save work, update Runtime, fully exit ChatGPT/Codex and retry.
The one-click flow performs repair and confirmed-model fallback automatically.
No account, auth.json or session deletion.

The v0.1.123 candidate workflow was cancelled before publication when final
review found that model preparation precedes per-Agent configuration. Version
0.1.124 repairs existing Codex drift before the Windows catalog transaction,
so the initial one-click model preparation cannot be blocked by the old route.
