# Runtime 0.1.82 Integration

The integrated release was advanced to 0.1.82 because another task created
v0.1.81 at video-only source 4f7f51a during integration. That existing tag is not
rewritten. This file retains its initial planning filename for traceability.

User authorized publication plus review/integration of the latest Runtime worktrees.
Scope is the Runtime repository; no Router/Console production deployment or
unrelated repository merge is part of this request.

## Worktree Review

At inventory, Runtime had ten registered worktrees. The canonical main was clean.
All worktree changes were read without switching, stashing, resetting or committing
inside another task's worktree.

| Branch | Reviewed State | Disposition |
| --- | --- | --- |
| `feat/tapnow-agent` | `97bbfc7` | Include one-click update/API replacement and target-version receipt checks |
| `fix/remove-video-trial-prompt` | Uncommitted at inventory, subsequently committed/pushed as `4f7f51a` by its task | Merge origin/main, retaining removal of trial prompt/copy action/non-persistent selector |
| `feat/runtime-video-models` | `56bac2f` | Already included in main |
| `feat/team-vectors-20261114` | `7d00ed0` | Already included in main |
| `feat/work-review` | `edd05d5` | Already included in main |
| `fix/preference-understanding-20261113` | `6f55a0b` | Already included in main |
| `fix/runtime-fable51-recommendation` | `521966e` | Already included in main |
| `fix/runtime-price-loading` | `bb1ad9f` | Already included in main |
| `feat/private-usable-memory` | `f554b32` | Already included in main |
| `fix/runtime-download-speed-20261113` | `97caae9`, `0f3561b` are not ancestors | Retain as historical alternatives; do not blindly merge |

The download-speed branch's own ACCEPTANCE-0163.md explicitly says its diagnostic
window-selection changes were not selected, failed/canceled 0.1.63 was superseded
by 0.1.64, and all functional download acceleration was preserved. Current main
already has the accelerated update code and succeeding validation. The two unique
commits add old diagnostics and a historical report, not missing shipping features.
They remain on the original branch; no history or uncommitted work was removed.

## Release Scope

Plugin 0.1.23 ZIP bytes and catalog remain unchanged. Runtime 0.1.82 now fetches
latest plugin metadata on every primary configuration action, verifies/prepares
the selected edition and uses the selected ACU Key/model. Successful receipt must
match the prepared version. Native browser regression seeds an old Key and checks
it is replaced while user rules remain. Backup must receive no Key.

The existing guide's Continue action resumes prepared files without downloading
again. First Chrome loading and subsequent Reload remain browser confirmations.
No silent replacement of other extension IDs or direct Chrome storage writes are
claimed. Latest metadata is publisher catalog metadata, not an inference about
unpublished upstream source.

Tests: Go/race/vet, frontend units, video-browser tests, TapNow guided-browser tests
and real native-host tests. Platform CI must pass Windows x64 and macOS arm64/amd64
before publication. Record final source SHA, workflow and asset hashes separately
from later documentation-only main advances.

## Concurrent Work After The Release Snapshot

After 0.1.82 CI completed, the preference-prewarm task advanced main to `ac7c130`
and started v0.1.83 workflow 43693425846. That commit is a descendant of the full
integration and retains all TapNow/video changes. It was reviewed and fast-forwarded
into this task; five prewarm tests, all 130 frontend tests, Go race checks and
the eight-layout preference browser suite passed locally. Canonical main and this
task branch were synchronized to that source without rewriting other worktrees.

The concurrently created `feat/video-reading-mcp` worktree remained dirty and
actively developing during publication: dependency/launcher files and subsequently
backend changes had no reviewed commit or completed acceptance. Those files were
not copied, committed, reset or merged by this task. They are explicitly retained
as unfinished development, not silently claimed as part of this release. The
original ten-worktree inventory is a point-in-time snapshot, not a claim that no
other worktree can appear while agents continue working.

## Publication Complete

Runtime 0.1.82 published at 2026-11-15 08:17:06 UTC from `45a49b7`, workflow
43692684220. Runtime 0.1.83 then published at 08:30:58 UTC from
`ac7c1302791c64a478b11e3413f73545923cd239`, workflow 43693425846. Both workflows
passed Windows x64, macOS arm64/amd64 and their automatic publication jobs.
GitHub latest and Console `/api/acu-runtime/release` returned 0.1.83 after
publication. All five installer digests in public SHA256SUMS matched GitHub asset
digest metadata. No Router/Console restart or deployment was performed.

0.1.83 installer SHA-256:

| Asset | SHA-256 |
| --- | --- |
| Windows x64 Setup.exe | `71a730718d20b99027bda4b2ada089bb42c4580194b4b425bbf37d271e92bc66` |
| macOS arm64 DMG | `709bddc9f396bb0d1d57c2ebf1b982bf25200d67739cd851c7c77f705aeb0784` |
| macOS amd64 DMG | `e282e5519ee3b6f15e41ad66b7f39ad62ce5a939c2d0817230127a0917b67315` |
| macOS arm64 app.zip | `f0688be7a647c0784f4f239984377a9f3882a0a4a3ea41ec149bc2a39f363c7c` |
| macOS amd64 app.zip | `ecfd4213c4cbcb4a5df567af6c76232dca705bc54f7f3f2bae5eb92a9e7b179d` |

Release: https://github.com/jerry0012009/acu-runtime/releases/tag/v0.1.83
Subsequent changes in this closeout are documentation only, not new application
bytes. Canonical local main, remote main and the integration task are synchronized
after recording publication; historical/active worktrees are not force-reset.
