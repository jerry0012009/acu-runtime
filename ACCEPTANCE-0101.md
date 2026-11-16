# Runtime 0.1.101 Workbench Hierarchy And Native Catalogs

TapNow Companion owns its model selector, connection state, sequential setup,
actions and completion. The main heading is TapNow Companion, while connection
is a smaller step heading. Asset Backup is secondary and folded initially;
its expansion is retained across status redraws. Existing Native Messaging,
receipts, packages and model selection behavior are unchanged.

Install Other Agents retains the existing WorkBuddy/ChatGPT desktop installers
and adds a direct entry into Magpie's own Agent installation directory. Its
original installer commands, copy controls and plugin-agent entry are reused;
Runtime does not run another set of CLI installers. Tools/MCP opens Magpie's
Library on the MCP tab, including its market, installation and agent assignment.
No second registry, market, package executor or config writer is introduced.

The kernel's URL cleanup could remove deep-link parameters before deferred
Runtime integration ran. Runtime now snapshots its original query in the
existing head bridge, uses bounded native-control navigation and retains the
existing user-confirmation/mutation paths. Kernel source/binary is unmodified.

Verification:
- Go tests/race/vet and 135 Node tests passed before final integration.
- Native deep-link regression tests cover URL cleanup, install expansion,
  MCP-tab activation, no duplicate clicks and existing model/disconnect flows.
- TapNow browser script exercises all three setup steps, waiting, cancellation,
  receipt completion and same-version reconfiguration at 320/390/768/1440 in
  both themes. Companion model is inside Companion; Backup stays collapsed.
- Official Magpie 0.1.4248 Linux CLI downloaded from its release and checked
  against SHA256SUMS: `786450eeac72f509174d7a81295cdde8cf3898f3d8d93b2615bd2fe8c9742bc3`.
- Real Runtime proxy/embedded official GUI opened the native install directory
  and MCP market. Context7 was installed through the original market button
  into a disposable HOME/CODEX_HOME/CLAUDE_CONFIG_DIR, then the fixture exited
  and temporary files were removed. No existing Agent credentials/config changed.
- Screenshots: `.build/tapnow-browser/`, `/tmp/acu-workbench-agent-install.png`,
  `/tmp/acu-workbench-mcp-installed-{1440,390}.png`. Browser fixture can be rerun
  with `MAGPIE_BIN`, `PLAYWRIGHT_MODULE`, `ACU_RUNTIME_TEST_EXE` via
  `scripts/test-workbench-tools.cjs`.

Main concurrently received account billing improvements in 0.1.100; retain
them and release this separate change as 0.1.101. Production Router/Console
are not changed by this Runtime-only release.
