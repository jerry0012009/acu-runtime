# Runtime 0.1.113 Mac In-App Updates

Adds architecture-specific official app.zip selection for Mac update checks.
Downloads reuse existing public mirrors and full SHA256 validation; application
identity, bundle version, ad-hoc signature integrity, Mach-O architecture and
packaged kernel manifest/version are verified before quitting. No claim of
Developer ID signing or notarization is made.

A detached copy of the current executable waits for the exact old PID/start
identity to exit. A same-volume staged application replaces the old bundle
with a full backup retained until the new PID reports desktop ready, expected
version and expected kernel version. Launch failure restores the old bundle.
Unknown/busy kernel state prevents handoff. Writable bundle parents only;
mounted DMGs, translocated paths and linked app roots are rejected. No sudo,
password capture, permission bypass or user configuration migration.

Settings no longer repeats Agent installation/update information. Existing
Workbench controls remain. Older Mac clients must install this release manually
once, because earlier clients cannot run the new updater themselves.

Tests cover exact Mac architecture/package matching, invalid release metadata,
unsafe archive paths/symlinks, native helper PID handoff and rollback, user config
preservation, and Settings controls. Native Mac tests run against the signed
packaged application on both Intel and Apple Silicon. Cross compilation is not
reported as native acceptance. Platform results follow after CI.
