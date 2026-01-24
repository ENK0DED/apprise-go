# PROCESS.md

Goal
- Build a Go implementation of Apprise with a drop-in `apprise` CLI, single-file static builds, and parity tests against the Python package.

Current status
- Scaffolded Go module and a minimal CLI with version output.
- Pinned Go version string to the installed apprise version (1.9.7).
- Added runbook (`AGENTS.md`) and `.gitignore`.

Environment setup
1) Python apprise for parity checks:
   - `cd apprise-go`
   - `python3 -m venv .venv` (already created)
   - `source .venv/bin/activate`
   - `python -m pip install apprise`
2) Go tooling:
   - `go version` should be available in PATH

CLI parity basics
- `apprise --version` should match the Python output:
  - `Apprise v<version>`
  - `Copyright (C) 2025 Chris Caron <lead2gold@gmail.com>`
  - `This code is licensed under the BSD 2-Clause License.`

Planned PR stack
1) Scaffold module + CLI + version plumbing + process docs. (In progress)
2) Add localhost capture server + parity test harness (shells out to Python apprise).
3) Implement URL parsing + `json://` plugin + minimal CLI flags needed by tests.

Next steps
- Add capture server and test harness under `apprise-go`.
- Implement JSON notifier and URL parsing in Go for initial parity.
- Expand CLI option handling to match apprise usage.

Notes
- Version updates: edit `internal/version/version.go` or override at build time with:
  `-ldflags "-X github.com/unraid/apprise-go/internal/version.Version=<version>"`
