# Empty Preference Example

The personal preferences page shows a read-only 50-direction example across five
domains only after both profile and library reads confirm an empty account.
Loading, failed reads, signed-out sessions, and team spaces retain their own states.

The title and visible note identify example data. Example directions have no
source records and never enter settings, controls, learning, or prompt injection.
The existing Private ACU switch remains the only opt-in control. A refreshed
personal profile replaces the example, even when it contains only one direction.

Validation: `node --test tests/preference-starfish.test.cjs tests/preference-prewarm.test.cjs`
and `scripts/test-starfish.cjs` cover geometry, account isolation, empty-only
examples, no writes from example interactions, replacement by actual preferences,
keyboard selection, mobile/desktop light/dark layouts, and reduced motion.
