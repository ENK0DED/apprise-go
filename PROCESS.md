# Development runbook

The Go port targets Apprise 2.1.0. `internal/version/version.go` records the
upstream compatibility version separately from the Go package release version.

## Current implementation

All upstream schemas are registered except `blink1`, which requires a USB HID
library and is explicitly listed in `internal/notify/unsupported.go`.

The 2.1.0 update includes AMQP 0.9.1 and 1.0, Cliq, DeltaChat, GitHub,
SimplePush's new encrypted API, and the new incident, SMS and push providers.
It also updates existing providers and their schemas, URL validation, request
fixtures, format negotiation, CommonMark dialects, Unicode overflow handling,
retry delivery tracking, notification deadlines, VAPID subscriptions and PGP/MIME.

Parity tests compare complete Python and Go request sequences. Captured HTTP
responses are controlled fixtures; passing these tests does not establish live
service acceptance. SMTP, IRC, MQTT, XMPP and AMQP tests use local protocol
servers. Independent crypto checks verify Matrix and PGP messages rather than
normalizing away their encrypted content.

## Environment

- Go: use the version in `go.mod` and set `GOCACHE=$PWD/.gocache`.
- Python reference: clone `caronc/apprise` at the version in
  `internal/version/version.go` into `../apprise`.
- Run `bash scripts/ci/setup_parity_env.sh` to create `.venv` with the pinned
  reference and capture dependencies, including AMQP 1.0 support.
- AMQP integration tests need a RabbitMQ broker supporting both wire protocols.
  Set `APPRISE_AMQP_TEST_ADDR`, `APPRISE_AMQP_TEST_USER` and
  `APPRISE_AMQP_TEST_PASSWORD`. CI supplies a bounded RabbitMQ service.

## Validation

```sh
GOCACHE=$PWD/.gocache go test ./... -count=1 -timeout=45m
GOCACHE=$PWD/.gocache go test -race -shuffle=on ./... -timeout=45m
GOCACHE=$PWD/.gocache bash scripts/ci/run_parity_tests.sh
```

The parity script checks generated Markdown vectors, writes
`reports/parity_report.md` and `reports/parity_report.json`, then checks runtime
schema capabilities, attachments and the broader option matrix. Review the
report's framework argument notes as well as provider and schema results.

Provider fixtures live in `internal/parity/providers/<provider>/`. Regenerate
selected goldens with `.venv/bin/python internal/testutil/scripts/update_golden.py
<provider>`; omit the provider to regenerate all. Use `--check` for verification.
Regenerate Markdown vectors with `.venv/bin/python
internal/testutil/scripts/generate_markdown_vectors.py`.

Capture caches live under `.tmp`. Set `APPRISE_CAPTURE_CACHE=0` for a fresh
request capture or `APPRISE_CASES_CACHE=0` for fresh schema cases. Increment the
capture contract version when capture behavior changes. Never normalize a
cryptographic value without an independent check of the actual wire behavior.

## Releases

Knope prepares the version and changelog in its release PR. Qualify that exact
head before merging. Merging the release PR creates the release; check the
result before dispatching another release workflow. Verify all eleven uploaded
assets, embedded source revisions, macOS signatures and notarization results.
See `.github/workflows/release.yml` for the guarded existing-tag recovery path.
