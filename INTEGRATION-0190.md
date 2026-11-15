# Runtime and TapNow Closeout, 2026-11-15

The user reported successful TapNow installation and requested main integration
and local/remote synchronization. This closeout changes documentation only; it
does not publish another application version or restart production services.

## Reviewed Sources

Runtime main at inventory: `2d5d770d4d48a2e52d905aa5d568275360c0dc9d`.
TapNow main at inventory: `0cc704525c91f2bb8c626398e56c55ae952504e3`.
All 15 Runtime and three acu-tapnow registered worktrees were clean. Every
registered feature branch and remote branch was checked against fetched main.

| Runtime branch | HEAD | Disposition |
| --- | --- | --- |
| `feat/tapnow-agent` | `2d5d770` | Integrated; owns this closeout |
| `feat/video-reading-mcp` | `2d5d770` | Integrated |
| `feat/runtime-media-workbench` | `16978c8` | Integrated |
| `feat/native-video-default` | `8fe2f8f` | Integrated |
| `fix/preference-prewarm-20261115` | `ac7c130` | Integrated |
| `fix/preference-understanding-20261113` | `6f55a0b` | Integrated |
| `fix/runtime-fable51-recommendation` | `521966e` | Integrated |
| `fix/runtime-price-loading` | `bb1ad9f` | Integrated |
| `feat/private-usable-memory` | `f554b32` | Integrated |
| `fix/remove-video-trial-prompt` | `4f7f51a` | Integrated |
| `feat/team-vectors-20261114` | `7d00ed0` | Integrated |
| `feat/runtime-video-models` | `56bac2f` | Integrated |
| `feat/work-review` | `edd05d5` | Integrated |
| `fix/runtime-download-speed-20261113` | `0f3561b` | Historical alternatives retained |

Only the download-speed branch has commits outside Runtime main: diagnostic
test change `97caae9` and historical acceptance report `0f3561b`. Its own report
explicitly records that the diagnostic change was not selected and failed 0.1.63
was superseded by 0.1.64 with all download acceleration preserved. The earlier
[integration review](INTEGRATION-0181.md) remains applicable. No old behavior is
restored just to make every branch an ancestor of main.

Both TapNow task branches are already ancestors of plugin main. This task's
`fix/tapnow-official-endpoints` worktree was fast-forwarded from `9f4685b` to
`0cc7045` before rerunning its tests. Other tasks' directories and branch heads
are retained without reset, stash, deletion or new commits made on their behalf.
The original `jerry0012009/tapnow` checkout remains an upstream reference, not
the acu-tapnow distribution source.

## Published Versions

- Runtime 0.1.90: application source `524dcaa`, successful desktop release
  workflow [43711527439](https://github.com/jerry0012009/acu-runtime/actions/runs/43711527439),
  published 2026-11-15 12:11:29 UTC. Windows x64 and macOS arm64/amd64 passed;
  all five desktop packages, installation notes and digest list are present.
- TapNow 0.1.27: package source `d34c979`, successful release workflow
  [43710773745](https://github.com/jerry0012009/acu-tapnow/actions/runs/43710773745),
  published 2026-11-15 11:49:45 UTC; catalog activation is `0cc7045`.
- Main can advance through acceptance/closeout documentation without rebuilding
  these published application bytes. Do not identify the documentation SHA as
  the source of an already-published installer.

## Verification

Fresh checks on the integrated sources passed Go race tests, go vet, all 132
Runtime JavaScript tests, all 92 plugin tests, zero production npm audit
vulnerabilities and both Chrome extension edition builds. The user reports
successful installation; real paid-model video review is not inferred from it.

The guided browser regression passed every setup step at 320/390/768/1440px
in light and dark themes, including GPT-6.1 Sol selection, automatic copying,
same-version reconnect and receipt completion. The actual Chromium MV3/native
host regression passed with plugin 0.1.27: replaces the old fixture Key, retains
user rules, reads configuration back and confirms the Runtime receipt. Backup
receives no Key and neither connection sends cloud requests. These use isolated
profiles and do not configure a real user's browser or charge model usage.

Canonical local main is fast-forwarded only after a successful non-force push.
Historical worktree heads intentionally remain at their original commits;
being behind main does not mean their completed features are missing from main.
