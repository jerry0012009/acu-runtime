# Runtime 0.1.81 Integration

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

Plugin 0.1.23 ZIP bytes and catalog remain unchanged. Runtime 0.1.81 now fetches
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
