# Runtime 0.1.102 Frontier Model Recommendations

Fable 5.2 was already in the curated array but absent from per-Agent example
chips, could be excluded by a stale kernel option list, and repeated setup
only revealed recommended hidden models for WorkBuddy. Provider refresh was
performed later on confirmation, after the user had already seen stale choices.

Explicit recommended setup now uses Magpie's existing provider/models refresh
before reading choices. It does not fetch secrets, invent model IDs or expand
Key permissions. Agent recommendations accept current native options or current
Agent catalog entries. Repeated setup reveals supported curated models for all
four Agents without changing existing primary model or unrelated visibility.
Fable 5.2 is a leading visible recommendation, including per-Agent example
chips. Gemini4 Carbon/Hydrogen are added when present in the selected Key's
catalog. acu-auto remains the default; no ongoing session model is replaced.

137 Node tests and Go race/vet passed before final integration. Browser checks
cover Codex, Claude Code, Hermes and WorkBuddy, fresh and existing setups,
390/1440 widths, both themes, Fable5.2/Gemini4 entries, confirmation and native
visibility writes. No second model catalog or installer is introduced.

Release v0.1.102 (application SHA 0601742) completed all three native build
jobs in GitHub Actions run 43847864537. Windows fresh installation, upgrade
from 0.1.70, native session renewal and packaged kernel compatibility passed;
both macOS architectures passed packaging and kernel compatibility. Published
at 2026-11-16T10:32:43Z with Windows Setup, two DMGs, two app ZIPs and checksums.
The public https://acu.best/api/acu-runtime/release endpoint returned 0.1.102
and matching platform URLs. Recommendations browser checks (32 scenarios)
and 87 startup tests also passed after integrating main SHA 5886989, which
retains this fix and separately prepares account-loading release 0.1.103.

Server supply investigation is in Router's
`docs/operations/2026-11-16-gemini4-vertex.md`. Heju fixed Vertex Gemini4 Profiles
are configured but currently upstream-rejected; Wawazz Anti Carbon and Grok4.7
were successful live samples. Recommendation is selection/access, not a claim
that every supplier is healthy. Existing model-restricted Keys remain restricted.
