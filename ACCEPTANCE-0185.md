# Runtime 0.1.85 Endpoint And Companion Default Fix

User reported TapNow setup failed before installation with the combined
unsupported-source/model error, and requested GPT-6.1 Sol rather than acu-auto.

## Root Cause

Runtime account login saves the selected official site as its provider base.
New logins use `console.acucompute.com`, but TapNow's narrower adapter allowed only
`api.acucompute.com` and `acu.best`. The plugin import and inference checks plus
manifest host permissions had the same mismatch. Existing tests used only acu.best,
so native-message success did not cover the default logged-in installation path.

## Fix

Runtime's adapter reuses `acuBase`: HTTPS official acucompute.com/acu.best origins,
official subdomains, existing `/acu/v1` paths, whitespace/trailing-slash normalization
and default HTTPS port support. It retains the exact provider origin; it does not
substitute domains or copy credentials to an unrelated endpoint. Plugin 0.1.24 has
matching URL validation and host permissions. Lookalike domains, URL credentials,
queries, fragments and nonstandard ports remain rejected. Native transport's
loopback checks are unchanged. Key/model/source errors are separated without
printing credentials.

TapNow model selection independently prefers `gpt-6.1-sol` if the selected Key
offers it. Missing Sol requires explicit model choice, never silent acu-auto
fallback. Explicit choices survive rerenders; a different Key uses its own model
list. No provider model, credential, other Agent or account file is changed before
the user's configuration action.

## Evidence

Local: 132 frontend tests, Go race tests/vet, two model-specific tests, eight
guide viewport/theme checks; plugin 87 tests and ZIP build/identity verification.
Real isolated MV3 -> native host -> Runtime -> local storage/receipt now uses
`https://console.acucompute.com/v1` with `gpt-6.1-sol`, seeds an old Key and retains
user rules, without cloud/inference requests. Release CI uses the same scenario
on Windows x64 and both macOS architectures, with exact plugin ZIP hashes.

Plugin source: `1ecae85fa00b2620db515eb50482ab038f5a99d0`, workflow 43698861725.
Published plugin 0.1.24 assets were downloaded and rehashed before catalog activation:

- Companion: 45,160 bytes, `84134ffdd6364d4bb3c8b9d8a1620a40a126af6bf05f262cdb64c9cdd4bb986a`.
- Backup: 189,047 bytes, `103c1eef41635cfcc462d3a7215e23775929df25956c9b729f9f3cb0c3c7fab8`.

This is configuration/compatibility acceptance, not a live paid Sol review, user
Chrome profile inspection or actual owner-device installation. Existing plugin
content/LLM TypeScript diagnostics remain. Browser permission prompts and Reload
for updated unpacked extensions still apply. No production Router/Console restart
or configuration change is required.
