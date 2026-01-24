# AGENTS.md

This repo is the Go port of Apprise. Use this file as a quick runbook when resuming work.

Project layout
- Python reference: `../apprise`
- Go port: this directory (`apprise-go`)

Environment
- Python virtualenv: `apprise-go/.venv` with `apprise` installed
- Go module: `github.com/unraid/apprise-go`
- CLI binary name: `apprise` (drop-in goal)

Workflow
- Use Graphite (`gt`) for stacked PRs.
- Tests should compare Go behavior to the installed Python apprise using a local capture server.
- Keep Go version strings aligned with upstream apprise (see `internal/version/version.go`).

Process log
- See `PROCESS.md` for current status, setup, and next steps.
