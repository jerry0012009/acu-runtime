# ChatGPT Desktop Update Verification And UI

## Findings

Read-only vendor checks on November 13 found windows-store-update.json advertising
26.1111.2228.0 while the official complete MSIX HEAD metadata and mirror manifest
still identified 26.1106.6628.0. The mirror explicitly marked the Windows artifact
currentForCodexVersion=false. This is a confirmed distribution mismatch, not proof
that the user's proxy caused or fixed their local state. No PC credentials, apps,
proxy settings or conversations were changed here.

Magpie's native /api/agents/cli update updates the Codex CLI through its original
installer; its App field is informational. Source review found no ChatGPT Windows
MSIX updater to substitute. CLI updates remain on Magpie. Runtime retains Windows'
official Add-AppxPackage deployment with publisher/signature checks; it does not
implement a new updater protocol or synthesize a desktop version.

## Fixes

- Known-stale official complete packages fail before a large download; package
  manifest version is checked against the requested target before installation.
- Windows deployment verifies registered version >= package version; Runtime
  verifies registered version >= requested update and invalidates the old cache
  on completion/failure. Missing/invalid registrations do not count as success.
- Mirror artifacts explicitly not current cannot become a false latest result.
- Successful UI refresh removes completion text and the old update offer; normal
  status polling cannot restore the old completion banner. Failed latest checks
  preserve a previously confirmed update rather than implying latest installed.
- First-install choices/progress and per-app update/error state are separate.
  No WorkBuddy recommendation appears during a ChatGPT update. Retry targets the
  actual selected app. Cancel remains available before deployment.

## Verification

Go tests cover stale/exact/newer/missing installed versions, stale package content,
HEAD preflight, download identity and mirror validation. JS tests cover successful
and failed desktop updates, polling cleanup, network check failures and separated
installation/update guidance. Browser screenshots cover choose/install/update/error
in light/dark at 390/1440 widths. Release CI also checks native Windows startup,
fresh official ChatGPT package registration, old Runtime upgrade and kernel API.

The live read-only metadata acceptance reproduced and blocked the exact stale
26.1106.6628.0 package against target 26.1111.2228.0. Installing that target on the
user PC is not claimed while the complete vendor package remains behind. The
client's own updater remains the available vendor-managed alternative.
