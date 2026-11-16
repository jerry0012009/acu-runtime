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

Desktop release verification and publication evidence will be recorded here
after all three platform jobs complete.
