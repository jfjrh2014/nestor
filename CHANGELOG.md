# Changelog

## [Unreleased]

### Added
- All 19 commands: up, sync, diff, add, list, edit, dashboard, doctor, rollback, snapshot, restore, import (chezmoi/yadm/brewfile), ci, profiles, remote (add/show/remove), secrets (inject/check), and version.
- Dotfile management with copy/symlink strategies, simple Go templating, and drift detection.
- Package bootstrap per-OS (macOS/brew, apt, dnf, pacman, snap) with WSL detection.
- Secrets injection via 1Password, Bitwarden, Vault, or env providers — keys only in config, never values.
- Automatic snapshots before every `up`, with prunable history and one-command rollback.
- Interactive bubbletea dashboard (`nestor` with no subcommand).
- Remote config sync over git: `nestor remote add/show/remove`.
- CI-safe config validation: `nestor ci`.

### Fixed
- Dead-value bug family (sessions #22–#56): computed statuses, counters, and styled output that were discarded instead of used — in diff, sync, secrets check, secrets inject, and ui.Info rendering.
- Six commands swallowed errors and exited 0 on total failure; all now propagate real exit codes.

### Engineering
- 325 tests across 13/13 packages; every internal package 85%+ statement coverage (most 90%+).
- go vet, staticcheck, `go test -race`, and gofmt gates all clean; `make fmt` / `fmt-check` targets.
- Cross-compile matrix verified: linux/amd64+arm64, darwin/amd64+arm64, windows/amd64.
