# Runtime 0.1.65 Native Update Authority

ChatGPT desktop is no longer compared to an independent official or mirror latest
feed during Runtime update checks. The installed version is informational only;
it has no Latest/Update claim. Magpie's Codex CLI response, including update=false,
update=true, installer and desktop-version fields, remains unchanged. Runtime's
own updates and WorkBuddy's existing desktop flow are outside this narrow change.

ChatGPT desktop is excluded from per-Agent update buttons, bulk updates and stale
update-status rendering. Its old manual action was removed. Both the local HTTP
installation endpoint and direct worker reject ChatGPT update requests before
version probes, downloads or configuration writes. First installation still uses
the existing signed package path; no user app/configuration is removed or rewritten.

Tests cover exact preservation of Magpie's update decision, no independent desktop
feed access, bypass rejection, no UI offer even for a stale injected desktop
update=true, ignored historical failure messages, native CLI updates still using
the kernel endpoint, and initial-install availability. Browser tests check supported
update/install flows and absence of ChatGPT desktop update controls across themes
and widths. Native Windows and three-platform packages are release CI gates.
