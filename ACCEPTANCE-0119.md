# GPT-6.2 One-Click Configuration

Runtime 0.1.119 includes exact `gpt-6.2-sol` and `gpt-6.2-astra` in the
recommended model inventory, setup preparation and Agent summaries. Only models
available to the chosen Key and Agent are offered. Codex, Claude Code and
Hermes can select either explicitly; WorkBuddy keeps in-app model selection.
Existing configured defaults, original models, credentials and model limits
remain unchanged. Reapplying recommended setup refreshes the native catalog and
reveals both models. No upstream profile, pricing or Auto policy is changed.

Local verification:

- `go test ./...`, `go vet ./...`: pass.
- `node --test tests/*.test.cjs`: 156 pass, including exact-model configuration
  and unavailable-Key filtering for each new model across all four Agents.
- Playwright recommendations: 32 combinations (four Agents, fresh/existing,
  light/dark, 390/1440 widths), exact selection, preservation, visibility,
  preparation requests and no horizontal overflow. Screenshots inspected.
- Linux native desktop-tag test/vet could not compile because GTK4/WebKit6
  development libraries are absent. Windows/macOS native packaging and native
  kernel compatibility remain the formal release workflow's responsibility.
- No paid inference or live user Agent configuration performed by these tests.

Screenshots: `/root/jerry/exports/gpt62-entrypoints-20261118/output/runtime-recommendations`.
The official Magpie v0.1.4350 kernel is unchanged. Its isolated native model
sync fixture now includes both exact GPT-6.2 IDs for the release runners.
