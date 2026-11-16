# Runtime 0.1.110 Price References

Price multipliers compare actual CNY payable rates with same-tier official
Standard/Fast reference rates converted using catalog FX. Supplier credit
conversion is not foreign exchange. Input/output discounts differ when supplier
rates diverge from official pricing, so the UI lists them independently.
Missing references never fall back to settlement multipliers.

The public Heju catalog explicitly identifies seven GPT Fast models as 2x
their named Standard model. The operator approved matching ACU static retail
to paired Standard retail x2. This policy is published centrally; Runtime
does not calculate user charges or alter historical records.

Prices are 16px; toolbars share one row when space permits; context thresholds
appear once per model. Public reference details prefer explicit server official
metadata. Unknown Fast references are omitted, not compared against Standard.

Verification includes Go tests/race/vet, Node tests, and billing browser fixtures
with real public catalog at 320/390/600/920/1280/1440/1920px, light and dark.
Existing 0.1.109 preference example, tools, Agents and pinned Magpie 0.1.4248
are retained. No private account or Agent configuration is changed by tests.

Release `v0.1.110` published at `2026-11-16T15:57:35Z`, application revision
`639bed92ef8b304685832badb14f7f56ede5597c`. Workflow `43898886284` completed
successfully for Windows x64, Apple Silicon, Intel Mac, and publication. The
initial Apple Silicon runner queue cleared without a retry or source change.
Windows native session renewal, clean-client installation, quiet Codex routing,
old Runtime upgrade and packaged-kernel compatibility all passed. Both Macs
passed native build, browser cases, package smoke and kernel compatibility.

The public `/api/acu-runtime/release` returns 0.1.110 with all three mirrors.
All five package artifacts are included in the published SHA256SUMS manifest;
the locally downloaded CI artifacts are compared with that manifest. Windows
installer SHA256 is `bb172835c2caff5e6eb280ea2e97012baedb0988c66c708bc88397b0778d674a`.

Static policy version `acu-fast-standard-2x-1794843378662` is effective; all
seven Fast pairs compare at exactly twice their paired Standard components,
two pairs preserve Standard context tiers and five retain flat retail. Ninety-one
unaffected model payable snapshots are unchanged. Supplier Profiles and historic
bills are unchanged by this retail policy. No cloud application restart was
needed for this Runtime/package and price-book publication.
