# Runtime 0.1.118 Codex Login Catalog Validation

## Confirmed Defect

The quiet Windows route transaction stopped the owned Magpie gateway, applied
the native login/model selection and asked the installed Codex app-server for
its model list before restarting the gateway. A signed-in beside route may
fetch that list through Magpie instead of a local model_catalog_json file.
Validation therefore ran against an unavailable dependency and could reject a
valid login-mode change, restoring the prior API preference.

The same added regression test run against unmodified 0.1.117 with a Go overlay
failed with connection refused at the paused loopback gateway. With the change,
the official Magpie v0.1.4350 and official Codex 0.169.0-alpha.11.1 isolated
transaction passed twice (67s and 73s). No customer credentials were used.

The transaction now restores the owned gateway before native validation. On
failure it stops that gateway, restores the existing route snapshot and starts
the old route again. Original model/auth/state assertions remain; the concrete
validation error is retained. User-selected api/chatgpt/automatic modes remain
unchanged by model selection, and login-mode changes must still pass validation.

## Verification

- Go test ./..., race suite and vet passed.
- 148 JavaScript tests passed; no frontend behavior change.
- Real official binaries: gateway ownership at validation, all three login-mode
  changes and retention across model selection, catalog batch operations, invalid
  model rollback, canceled validation rollback, disconnect and unrelated MCP
  preservation passed in a temporary Linux profile.
- Windows clean-runner native/installer checks are required before delivering
  the Windows artifact. Linux validation does not certify the user's desktop UI.

## Limits And Upstream Reuse

Magpie v0.1.4350 and CLIProxyAPI are unchanged. Native login commands still own
provider/auth routing. No account token refresh/login/logout is added by Runtime,
no workspace policy is stubbed, and no automatic global mode reset is performed.
The source of this user's initial saved API preference is unknown. Runtime once
forced that preference before 0.1.69, but current model selection does not do so;
there is no evidence identifying which writer changed this user's setting.

The reported gray/green avatar and new-chat workspace error are not reproduced
on the customer's machine. Magpie documents identity transitions in its native
provider requires_openai_auth handling (issue 6037); account discovery disabled
can also prevent the beside route (issue 6917). Neither is confirmed locally for
this user. Fixing the false catalog rejection enables the supported switch back,
but does not certify recovery from independent account/proxy/Cloudflare failures.

Recovery: save work, fully quit ChatGPT/Codex, install 0.1.118, explicitly select
automatic compatibility in Runtime and reopen the desktop. Do not erase auth,
sessions or .codex, force requires_openai_auth or disable workspace policy.
