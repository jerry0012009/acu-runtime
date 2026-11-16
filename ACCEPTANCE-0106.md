# Runtime 0.1.106 Agent Model Synchronization

Root causes:

- Runtime originally saved all then-listed provider model IDs as explicit
  picks. Magpie's native Fetch models refreshes the catalog, not those picks.
  Newly published common models could therefore be known but remain off,
  excluded from both Runtime recommendations and Agent catalogs.
- Windows Runtime rejects direct Codex catalog writes to avoid Magpie's live
  route watcher restarting ChatGPT's hosted app-server. The previous frontend
  recommendation fix still used this rejected path.

Recommended configuration now refreshes the native catalog, adds only requested
common IDs that actually exist in that catalog through `provider models +id`,
and verifies they are on. Existing picks and permissions are not replaced.
Visibility writes use a Runtime endpoint, native Magpie APIs, and a frontend
readback check before reporting success. Existing primary models are retained
by repeated bulk setup.

Windows catalog mutations use the existing owned-kernel idle transaction and
rollback snapshot. While its gateway watcher is stopped, Magpie's original
authenticated web management process performs its native writes; a reserved
gateway port prevents that temporary controller starting another gateway or
route watcher. It quits and settles its native sync before the owned gateway
returns. No Agent file schema or model catalog is reimplemented. Snapshots also
cover native settings/provider files for failed transactions. Running inference
must drain first; no user ChatGPT process is killed by Runtime.

Validation before release:

- Official Magpie 0.1.4248 isolated management test reproduces fetched-but-off
  Fable 5.2, adds Fable 5.1/5.2 and Gemini4 Carbon/Hydrogen through the original
  CLI, connects WorkBuddy through the original API and checks its actual
  `models.json` contains all five IDs including acu-auto.
- Official kernel + installed Codex config/model parser test passes catalog
  hide/reveal, watcher replacement, login-mode preservation, original MCP
  preservation, cancellation, rollback and disconnect. It makes no inference.
- Go race and vet pass; 137 Node tests pass.
- 32 browser cases cover four Agents, fresh/existing setups, light/dark and
  390/1440 widths, with visibility writes and readback. Screenshots private at
  `/tmp/acu-agent-model-sync-browser/output/runtime-recommendations/`.
- Release CI now repeats the isolated native-management test with each
  platform's packaged official kernel; Windows quiet-route smoke includes
  the new catalog hide/reveal transaction.

Native fixture rerun:

```bash
ACU_MODEL_SYNC_KERNEL=/absolute/path/to/official/kernel \
go test -run '^TestNativeCatalogManagementSync$' -v -timeout 3m .
```

This validates selection and configuration propagation, not supplier uptime.
An upstream failure must not make a published model disappear from the list.
