# Codex Final Configuration Outcome

The reported message originates from the Windows quiet transaction's command
failure branch, not an ongoing recovery progress stream. Recovery is synchronous
before the HTTP error returns, but the old text said it was still in progress.
This is confirmed from code; the user's actual command failure is not known.

Explicit native field/disconnect commands that exit unsuccessfully now receive
an exact native state readback while the owned gateway remains paused. Only a
matching target can continue through normal provider/tool postprocessing,
gateway recovery, installed Codex validation and unchanged-auth checks. A
canceled request, unknown mutation, mismatched field or real drift cannot take
this path. Arbitrary provider/catalog mutations always preserve their errors:
an already-working model is not proof of full catalog synchronization.

Successful rollback followed by gateway recovery now reports that the previous
configuration was restored and the requested change did not take effect.
Rollback failure, gateway restart failure and canceled requests remain distinct
failures. Existing auth/session/MCP preservation and snapshot rollback remain.

Verification covers exact GPT-6.2 readback, mismatch, wiring drift, disconnect,
cancellation, unspecified mutation and terminal rollback text. The opt-in official
Magpie + installed Codex isolation test includes a synthetic catalog mutation
failure and verifies unchanged config plus completed recovery wording. No live
user configuration or inference is used. Production release CI also verifies
Windows installed Codex, both macOS architectures and upgrade compatibility.
