# Runtime 0.1.61 UI Copy Review

## Scope

Reviewed Runtime-owned preference, graph, account, setup, billing, settings,
tray and embedded-host copy. Kernel-owned application text and saved user memory
documents are not rewritten by this UI change.

## Removed

- Repeated statements that saved preferences are not user quotations.
- Vector length/angle explanations and heuristic-confidence disclaimers.
- Estimated strength/confidence numbers in graph details; underlying geometry
  and interaction are unchanged, not relabeled as precise measurements.
- Source-document counts, routine graph timestamps, cache lifetime explanations,
  repeated account/Key boundary narration, and learning-outcome disclaimers.
- Repeated setup capability caveats and redundant model-selection explanation.
- Billing page introductions and repeated historical-price explanations; rows
  still distinguish actual charges from converted historical quota.

## Retained

- Always-expanded graph immediately below the page heading, with animation,
  selection, keyboard navigation, zoom and all directions accessible.
- Preference content and scope, initialized versus learned origin as short labels.
- Actual loading/error states, partial activity retrieval, session failure,
  unavailable capabilities, price units and estimated local costs.
- Confirmation and impact text for authorization, updates, restart, disconnect,
  exit and configuration replacement.

## Verification

Regression checks reject the removed disclaimer strings in rendered preference
markup, including collapsed details. They verify graph-first placement and no
outer disclosure after rerenders, retained partial-load errors and complete
ordinary/bipolar navigation. Browser cases cover both themes at 320, 390, 768 and
1440 pixels. Go tests and all 114 JavaScript tests passed locally. Release CI
also runs native desktop, installation, upgrade and packaged-kernel checks.
