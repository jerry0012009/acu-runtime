# Runtime Acceptance Of TapNow 0.1.31

2026-11-15: the user authorized formal publication of the ACU visual update.
Both editions are published at
https://github.com/jerry0012009/acu-tapnow/releases/tag/v0.1.31 .
The plugin main catalog was activated only after downloaded release bytes,
digests, sizes, fixed identities and every build entry were verified.

- Actual MV3/native host/Runtime acceptance passed using the downloaded public
  ZIPs. Configuration readback and receipts passed, user rules retained, Backup
  received no credential, and zero cloud/inference calls were made.
- `go test -run Tapnow -count=1 ./...` passed.
- `ACU_TAPNOW_RELEASE_VERSION=0.1.31 go test -run
  '^TestTapnowPublishedChannelPreparesBothEditions$' -count=1 -v .` passed.
  This uses Runtime's real public channel URL, HTTP client, digest verification,
  safe extraction and fixed identity checks; installs only into `t.TempDir()`.
- Native browser acceptance waits for the model input to be attached rather
  than visible, since the new popup intentionally collapses advanced settings.

No Runtime application code, version, installed user profile or backend service
changed. Existing compatible Runtime binaries fetch the updated channel when
checking/updating/configuring TapNow. Chrome requires users to Reload an already
loaded unpacked extension and refresh an existing TapNow tab. This does not claim
remote browser update, paid inference or real cloud-canvas acceptance.
