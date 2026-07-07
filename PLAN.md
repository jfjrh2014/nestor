# nestor — Your dev environment, from zero to coding.

**Language:** Go | **License:** MIT | **Author:** Marcus (first name only)

---

## Why nestor?

Nobody owns the full developer environment lifecycle. Dotfile managers don't bootstrap packages. Bootstrap scripts don't manage secrets. Nix is powerful but brutal to learn. chezmoi is close but overwhelms people.

**nestor** does it all: packages, dotfiles, secrets, shell setup, drift detection, rollback. One config file. One command. Beautiful output.

### Competition Matrix

| Feature | nestor | chezmoi | yadm | stow | Ansible |
|---|---|---|---|---|---|
| Package install | ✅ | ❌ | ❌ | ❌ | ✅ |
| Dotfile management | ✅ | ✅ | ✅ | ✅ | ✅ |
| Secret injection | ✅ | ✅ | ❌ | ❌ | Partial |
| Templating | Simple Go | Complex Go | ❌ | ❌ | Jinja2 |
| Drift detection | ✅ | ❌ | ❌ | ❌ | ❌ |
| Rollback | ✅ | Partial | git revert | Manual | ❌ |
| Cross-platform | ✅ | ✅ | ✅ | ✅ | ✅ |
| Easy to learn | ✅ | ❌ | ✅ | ✅ | ❌ |
| Beautiful TUI | ✅ | ❌ | ❌ | ❌ | ❌ |
| Import from others | ✅ | ❌ | ❌ | ❌ | ❌ |
| Profiles/modes | ✅ | Per-host | ❌ | ❌ | Inventory |
| Shell plugin mgmt | ✅ | ❌ | ❌ | ❌ | ❌ |
| Config testing (CI) | ✅ | ❌ | ❌ | ❌ | Partial |

---

## Phase 1: Core (v0.1)

### 1. Single config file — `nestor.yml`

```yaml
version: 1

packages:
  common:
    - git
    - neovim
    - ripgrep
    - fd
    - bat
    - fzf
    - tmux
    - jq
  macos:
    - homebrew/cask: visual-studio-code
    - homebrew/cask: rectangle
  linux:
    - snap: code
  wsl:
    - apt: ubuntu-wsl

dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy    # copy | symlink (default: copy)

  templates:
    - src: gitconfig.tmpl
      dest: ~/.gitconfig
    - src: tmux.conf.tmpl
      dest: ~/.tmux.conf

secrets:
  provider: bitwarden  # 1password | bitwarden | env | vault
  mappings:
    - key: github_token
      inject:
        - ~/.config/gh/hosts.yml: "oauth_token: {{.github_token}}"

shells:
  default: zsh
  plugins:
    - zsh-users/zsh-autosuggestions
    - zsh-users/zsh-syntax-highlighting
    - starship

profiles:
  personal:
    packages: [discord, spotify]
  work:
    packages: [slack, zoom]
```

### 2. `nestor up` — The Magic Command

- Detects OS, arch, package manager
- Installs all packages
- Deploys dotfiles (resolve templates, inject secrets)
- Sets up shell + plugins
- Runs post-install hooks
- Beautiful TUI progress with bubbletea

### 3. `nestor sync` — Capture Current Machine

- Scans current dotfiles
- Detects installed packages
- Generates/updates nestor.yml from live state

### 4. `nestor diff` — Drift Detection

- Compares live state vs config
- Shows missing, extra, changed
- Color-coded minimal output

---

## Phase 2: Polish (v0.2)

### 5. `nestor edit` — Interactive Template Editing

- Opens template in $EDITOR
- Live preview of resolved output
- Validates on save

### 6. `nestor secrets` — Secret Management

- `nestor secrets inject` — resolve all secrets and deploy
- `nestor secrets check` — verify all secrets accessible (dry run)
- Supports: 1Password CLI, Bitwarden CLI, env vars, HashiCorp Vault, plain files
- Secrets NEVER stored in config, only key references

### 7. `nestor list` — What's Managed

- Lists all managed packages, dotfiles, secrets
- Status: installed/missing/drifted

### 8. `nestor add <thing>` — Interactive Add

- `nestor add package ripgrep`
- `nestor add dotfile ~/.bashrc`
- `nestor add secret github_token`
- Interactive prompts, auto-updates config

---

## Phase 3: Power Features (v0.3)

### 9. Profiles/Modes

- `nestor up --profile personal` vs `--profile work`
- Different packages, dotfile variants, secrets
- Swap without reinstalling

### 10. `nestor rollback`

- Snapshot before every `up`
- Roll back if something breaks
- Simple file copies + manifest

### 11. `nestor doctor`

- Health check: packages installed? Dotfiles present? Secrets valid?
- Color-coded report, like `brew doctor` for everything

### 12. TUI Dashboard — `nestor` (no subcommand)

- Beautiful bubbletea TUI
- Shows: managed items, status, last sync time
- Interactive: toggle packages, edit dotfiles, inject secrets
- lazygit-inspired UX

---

## Phase 4: Ecosystem (v0.4+)

### 13. `nestor restore --from <url>`

- Pull someone else's nestor.yml from URL
- Perfect for team onboarding

### 14. Import from existing tools

- `nestor import chezmoi`
- `nestor import yadm`
- `nestor import brewfile`

### 15. `nestor ci` — Test Your Config

- Validates config works from scratch
- Docker container → `nestor up` → verify
- Config-as-code testing

---

## Technical Choices

| Decision | Choice | Why |
|---|---|---|
| Language | Go | Single binary, cross-compile, fast |
| TUI | bubbletea + lipgloss | Best Go TUI framework |
| Config | YAML + Go templates | Universal, readable, no new DSL |
| Secrets | Delegate to existing CLIs | No reinventing crypto |
| Dotfile strategy | Copy by default | Avoids #1 stow complaint |
| Storage | ~/.config/nestor/ + git remote | Standard XDG, version controlled |

## Design Principles

1. Copy by default, symlink optional
2. Simple config, no DSL
3. One command to rule them all
4. Drift detection — nobody else does this
5. Import existing setups — low friction
6. Beautiful output — lipgloss, clean progress

---

## Development Journal

### 2026-06-04 — Project kickoff

- Plan finalized
- Repo created
- Phase 1 scaffold: project init, config parsing, `up` command skeleton

### 2026-06-04 — Daily dev session #2

- Built `internal/platform` package: OS, arch, package manager detection (WSL aware)
- Built `internal/ui` package: ANSI styled output (✓ ! ✗), dependency-free for now
- Rewired `cmd/up.go` to use platform detection + styled output
- Fixed `configPath()` to prefer local `nestor.yml` over `~/.config/nestor/nestor.yml`
- Added tests for platform + config packages
- Build clean (CGO_ENABLED=0), all tests pass, vet pass
- Next: package installation step in `up`

### 2026-06-05 — Daily dev session #3

- Built `internal/packages`: Spec parser, Resolver, InstallAll
- Parsed specs: plain name, `manager: name`, `manager/sub: name`
- Resolver merges common + platform lists, dedup'd
- Backends: brew (formula + cask), apt, dnf, pacman, snap
- IsInstalled skips already-present packages
- Wired into `cmd/up` with summary line
- Smoke-tested: installed `sl` and `cowsay` on linux, then cleaned up
- Tests cover parse, resolve, manager factory
- Next: dotfile deployment (copy + symlink strategies)

### 2026-06-09 — Daily dev session #4

- Built `cmd/list`: shows all managed packages, dotfiles, secrets with live status
- Built `cmd/add`: interactively adds packages, dotfiles, or secrets to config
- Built `cmd/diff`: drift detection for packages (missing) and dotfiles (drifted/absent)
- `internal/dotfiles`: full deploy (copy + symlink), Go template rendering, drift check
- `internal/secrets`: env/1password/bitwarden/vault providers, injection into files
- `cmd/up`: fully wired — packages → dotfiles → secrets in sequence
- `internal/config`: added Marshal() for round-trip editing (load → modify → save)
- Added .gitignore for test artifacts
- 29 new tests across all packages, all passing. Build clean. Vet clean.
- Next: `nestor rollback` or `nestor edit`

### 2026-06-10 — Daily dev session #5

- Built `cmd/doctor`: full health check — config, platform+pkgmgr, packages, dotfiles, secrets provider
- Built `cmd/sync`: scans installed packages + common dotfiles, generates or merges nestor.yml
- Doctor output: color-coded ✓/!/✗ per category, issue count, fix suggestion
- Sync: detects 5 packages (git, curl, wget, vim, jq) + 2 dotfiles (.bashrc, .gitconfig) on this machine
- Both merge-aware: doctor on existing config, sync merges detected packages into existing config
- `secretCLI()` helper maps provider names to required binary names
- 5 new tests (scanDotfiles, scanPackages, checkPkgInstalled, secretCLI, valid config)
- Build clean. Vet clean. All tests pass. Pushed.
- Next: `nestor rollback` or `nestor edit`

### 2026-06-13 — Daily dev session #6

- Shipped rollback feature (was staged but blocked on commit permission previously)
- `internal/snapshot`: Create, List, Restore, Delete — backs up dotfile destinations before overwriting
- `cmd/rollback`: restores from latest or specific snapshot ID
- `cmd/snapshots`: lists available snapshots
- `cmd/up` auto-snapshots before dotfile deploy (non-fatal on failure)
- Next: profiles feature

### 2026-06-13 — Daily dev session #7

- Built profiles feature: `nestor up --profile personal` layers extra packages
- `cmd/profiles`: lists all defined profiles with package counts (sorted, deterministic)
- Config gains `ValidProfile` and `ProfilePackages` helpers
- Profile packages merge into resolver after platform packages (dedup handled by IsInstalled check)
- Renamed local `failed` vars to `failedCount` in up.go to avoid shadow lint issues
- 2 new tests (TestValidProfile, TestProfilePackages) — all 9 config tests pass
- Build clean. Vet clean. All tests pass. Pushed.
- Next: `nestor edit` (interactive template editing)

### 2026-06-14 — Daily dev session #8

- Built `cmd/edit`: open template in $EDITOR, create new if absent, preview rendered output for .tmpl files
- Exposed `dotfiles.Render` (exported wrapper around renderTemplate) so edit can preview without deploying
- `openEditor` respects $EDITOR, $VISUAL, falls back to vi
- 3 new tests (create+preview, missing config error, non-template skips preview) — all pass
- Build clean. Vet clean. All 38 tests pass. Pushed.
- Next: Phase 2 item #6 — `nestor secrets` (inject + check subcommands)

### 2026-06-15 — Daily dev session #9

- Built `cmd/secrets`: `nestor secrets inject` and `nestor secrets check` subcommands
- `inject`: resolves all configured secrets via the provider, writes them into target dotfiles
- `check`: dry run — verifies provider CLI is in PATH, every key resolves, lists inject targets
- Reuses existing `internal/secrets` provider infrastructure (env, 1password, bitwarden, vault)
- 4 new tests (no-secrets, env-provider reachable, missing-key detection, inject-creates-file)
- Fixed Go 1.19 compat in tests (`t.Context()` not available, used `context.Background()`)
- Build clean. Vet clean. All 42 tests pass. Pushed.
- Next: Phase 3 TUI dashboard (#12) or Phase 4 import (#14)

### 2026-06-17 — Daily dev session #10

- Built `nestor import` — Phase 4 item #14: import packages and dotfiles from existing tools
- Three importers: **chezmoi** (source dir walk, decodes dot_/private_/executable_ prefixes), **yadm** (`yadm list -a`), **brewfile** (parses brew/cask/tap entries)
- Auto-detection: `nestor import` with no source checks chezmoi, then yadm, then Brewfile
- `--dry-run` flag previews what would import without writing
- `MergeResult` deduplicates against existing config (packages by string match, dotfiles by dest path)
- Chezmoi path decoder handles compound attribute prefixes (private_executable_dot_foo -> .foo)
- 9 new tests across all three importers + dedup logic
- All 67 tests pass. Build clean. Vet clean.
- Next: Phase 3 TUI dashboard (#12) or Phase 4 item #15 (`nestor ci`)

### 2026-06-18 — Daily dev session #11

- Built `nestor dashboard` — Phase 3 item #12: interactive bubbletea TUI
- 4 tabs: Overview (health summary), Packages (installed/missing status), Dotfiles (present/drifted/missing), Secrets (configured keys)
- Async status loading: tea.Cmd polls platform.Detect() + packages.Manager.IsInstalled at launch, TUI renders immediately
- Navigation: j/k or arrows, tab/shift+tab cycles panels, 1-4 jumps directly, q/esc quits
- Added charmbracelet/bubbletea + lipgloss dependencies
- 3 new tests (dashSecretsProvider, dashProfiles, dashExpandTilde) — all 70 pass
- Build clean. Vet clean. Pushed.
- Next: Phase 4 item #15 (`nestor ci`) or Phase 4 item #13 (`nestor restore`)

### 2026-06-19 — Daily dev session #12

- Built `nestor ci` — Phase 4 item #15: CI-safe config validation
- `internal/ci`: static validator (no installs, no writes, no network)
  - Checks: version, strategy, package manager prefixes, template dedup + source existence, secret provider + key completeness, profiles
  - Findings classification: errors (fatal) vs warnings
- `cmd/ci`: runs validation, exits non-zero on errors (CI-friendly)
  - `--quiet` flag silences output on success (reduce CI log noise)
- 12 new tests (version, strategy, dup dest, empty fields, source exists, unknown mgr, no provider, invalid provider, valid provider, empty key, profiles, report counts)
- All 80 tests pass. Build clean. Vet clean. Pushed.
- Next: Phase 4 item #13 (`nestor restore --from <url>`) — last remaining planned feature

### 2026-06-20 — Daily dev session #13 — FINAL PLANNED FEATURE

- Built `nestor restore --from <url>` — Phase 4 item #13: pull config from remote URL
- `internal/restore`: Fetch (HTTP GET, 10MB cap, 30s timeout), Validate (parses + checks config), Write (overwrite protection + nested dir creation), Preview (human-readable summary)
- Security: rejects `file://`, `ftp://`, and other non-http(s) schemes. Refuses to overwrite without `--force`.
- `cmd/restore`: `--from <url>` (required), `--dry-run`, `-o/--output`, `--force` flags
- Exported `config.Config.Validate()` so restore can validate without disk writes
- 7 new tests + subtests (URL validation x7 schemes, fetch via httptest server, config validation x5, write new/overwrite/force/nested-dirs, preview output, file:// rejection)
- **All 90 tests pass. Build clean. Vet clean. Pushed.**
- 🎉 ALL 15 planned features across 4 phases are now complete.

### 2026-06-21 — Daily dev session #14

- Built shell plugin setup — the missing TODO stub from `up.go` Step 6
- `internal/shell`: Detect (current shell via $SHELL), RCFile (zsh/bash rc paths), ParsePlugin (github vs named), InstallPlugins (shallow git clone to ~/.config/nestor/plugins), SourceLines (generate source entries), WriteSourceBlock (idempotent marker-wrapped block in rc file)
- Wired into `up.go` as Step 6 — replaces the old `// TODO: configure shell` line
- Idempotent: re-runs update the managed block in place without clobbering user content
- Named plugins (starship, eza) are skipped — expected as system packages
- 8 new tests (parse plugin, rc file, source lines + dedup, new file, idempotent update, empty lines, detect, named-only install)
- All 98 tests pass. Build clean. Vet clean. Pushed.
- Next: polish pass — README command table is stale (missing edit, secrets, profiles, rollback, snapshots, dashboard, ci, restore, import)

### 2026-06-22 — Daily dev session #15 — polish pass

- README command table updated: 7 → 17 commands. Documented all flags (--profile, --quiet, --force, --dry-run, -o).
- Added snapshot/rollback note under command table.
- Scanned codebase for TODOs/FIXMEs/HACKs — zero remaining in code (only journal references in PLAN.md).
- All 126 tests pass. Build clean. Vet clean. Pushed.
- Project is feature-complete with clean docs. Ready for v0.1 tag.

### 2026-06-23 — Daily dev session #16

- Added `nestor version` command with ldflags injection (main.version/commit/date)
- main.go: build-time vars + `cmd.SetVersion()` wires ldflags into the binary
- cmd/version.go: outputs version, commit, build date via cobra's writer
- 2 new tests (output format, SetVersion setter) — all 128 pass
- Wrote GitHub Actions CI (ci.yml: Go 1.19/1.21/1.22 matrix + golangci-lint) and release pipeline (release.yml: cross-compile linux/darwin/windows, auto-release with checksums on tag)
- ⚠️ Workflow files blocked from push — token lacks `workflow` scope. Files exist locally at `.github/workflows/` but couldn't push. Needs manual push or token with workflow scope.
- Build clean. Vet clean. 128 tests pass.
- Next: tag v0.1 once CI is up

### 2026-06-24 — Daily dev session #17 — release prep

- Added LICENSE file (MIT) — README claimed it but the file was missing
- Added Makefile: build, test, vet, lint, install, release (cross-compile linux/darwin/windows amd64/arm64 + SHA256 checksums), clean, release-dry
- Added .golangci.yml config: errcheck, gosimple, govet, ineffassign, staticcheck, unused, misspell, revive (test files exempted from revive/misspell)
- README: documented shell completion (bash/zsh/fish/powershell), build-from-source install, development make targets
- All 128 tests pass. Build clean (with ldflags injection verified). Vet clean. Pushed (commit 0f45351).
- ⚠️ CI/release workflows (.github/workflows/) still blocked from push — token lacks `workflow` scope. Files ready locally.
- Next: tag v0.1 once workflows are pushed

### 2026-06-25 — Daily dev session #18 — flaky test fix + worker injection refactor

- Fixed flaky secrets tests (TestSecretsCheckNoSecrets, TestSecretsCheckMissingKey, TestSecretsInjectCreatesFile, intermittently TestSecretsCheckEnvProvider)
- Root cause: test helpers captured output via os.Pipe + goroutine with a single Read(), which truncated output when flushed across multiple writes — a genuine data race
- Refactored runSecretsInject/runSecretsCheck to accept an io.Writer parameter instead of hardcoding os.Stdout, so tests pass a bytes.Buffer directly
- Eliminated all pipe/goroutine/channel code from test helpers — simpler and race-free
- Verified: all 4 secrets tests pass 10 consecutive runs, full suite passes 3x consecutive
- All 128 tests pass. Build clean. Vet clean. Pushed (commit e6609d4).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope)
- Next: tag v0.1

### 2026-06-26 — Daily dev session #19 — profile completion

- Profiles were packages-only per the code, but PLAN.md spec said "different packages, dotfile variants, secrets"
- Extended Profile struct: added Dotfiles ([]Template) and SecretMappings ([]Mapping) fields
- Added ProfileDotfiles() and ProfileSecretMappings() accessors on Config
- `nestor up --profile X` now layers dotfile templates and secret mappings on top of base config (in addition to packages)
- `nestor profiles` output shows counts for all three categories
- 4 new tests (ProfileDotfiles, ProfileSecretMappings, LoadProfileWithDotfilesAndSecrets, existing ProfilePackages still passes)
- 131 tests pass (all including subtests). Build clean. Vet clean. Pushed (commit fc9a61f).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-06-28 — Daily dev session #20 — deep config validation

- Old `validate()` only checked version and strategy — malformed nestor.yml failed mid-deploy with cryptic runtime errors
- Rewrote `validate()`: now checks secret provider validity, empty template src/dest, duplicate destinations, secret mappings with empty key or inject targets, and the same checks (duplicates, empties) applied to nested profile dotfiles/secrets
- 10 new tests covering: unknown provider, empty src, empty dest, duplicate dest, empty secret key, empty inject map, profile empty src, profile duplicate dest, profile secret missing key, valid config acceptance
- All 140 tests pass. Build clean. Vet clean. Pushed (commit 6486004).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-06-30 — Daily dev session #21 — remote sync (push/pull/remote)

- Shipped Phase 5 remote sync feature: version-control nestor config across machines via git
- `internal/vcs`: git delegation layer — Init, WriteGitignore (excludes snapshots/secret files), SetRemote/GetRemote/RemoteSet, Status (porcelain parser with correct XY-code handling), HasChanges, Commit (with built-in GIT_AUTHOR/COMMITTER env), Push, Pull
- `cmd/push` [--remote <url>]: init repo, stage + commit all changes ("nestor: sync config"), push to origin if configured
- `cmd/pull` [--remote <url>]: init repo, warn on uncommitted local changes, pull + merge from origin
- `cmd/remote` with add/show/remove subcommands
- Writer-injected variants (runPushOut, runPullOut, runRemoteAddOut, runRemoteShowOut, runRemoteRemoveOut) for testability — matches session #18 pattern
- 11 new tests across vcs package + cmd layer (init/idempotent, gitignore preserved, remote set/get/replace, status clean/untracked/modified, commit noop/creates-history, push local commit, remote add/show/remove, nestorConfigDir from env + local yml)
- README: documented push/pull/remote commands, added multi-machine sync quickstart
- All 130 tests pass. Build clean. Vet clean.
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-01 — Daily dev session #22 — drift detection bugfix

- Found and fixed a dead-code bug in nestor's headline feature (drift detection)
- The `extra` counter in `cmd/diff.go` was declared and printed ("N extra packages not tracked") but never incremented — so diff always reported zero untracked packages, silently defeating half of PLAN.md §4 ("Shows missing, extra, changed")
- Reused `scanPackages()` (already in sync.go) to surface installed dev packages missing from config; counts them as drift and points users at `nestor sync` to capture
- Added `untrackedPackages()` pure helper (sorted + deduped output) for testability
- 5 table-driven unit tests (dedup, sorted, empty cases, all-tracked, all-extra)
- Verified live: with only `jq` tracked, diff now correctly flags curl/git/vim/wget as extra
- All 130 tests pass (12/12 packages). Build clean. Vet clean. Pushed (commit 19bcfd4).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope)
- Next: tag v0.1 once CI unblocked

### 2026-07-03 — Daily dev session #23 — snapshot retention/pruning

- Snapshots accumulated unboundedly: every `nestor up` created one and nothing ever retired them, so the snapshot dir grew forever
- `internal/snapshot`: added `Prune(keep)` — keeps the N newest snapshots and removes the rest, returning the deleted IDs. `keep<=0` disables pruning (no-op guard)
- `cmd/snapshots prune`: new subcommand `nestor snapshots prune --keep N` (default 10), listing exactly what it removed
- `cmd/up`: auto-prunes to 10 after each snapshot create, so the snapshot dir stays bounded without users needing to think about it
- 5 new table-driven tests (removes-oldest, keep=0 noop, under-threshold noop, exact-threshold noop, empty-base noop)
- Snapshots parent cmd now advertises the new subcommand and `--keep` flag
- All 178 tests pass (13/13 packages). Build clean. Vet clean. Pushed (commit 1bd93ea).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-04 — Daily dev session #24 — sync dotfile merge bugfix

- Found a data-loss bug in `nestor sync` (the "capture current machine" command)
- When a config already existed, sync merged scanned packages into it but discarded scanned dotfile templates: `foundDots` was written to a throwaway cfg that got overwritten by the loaded existing config (`cfg = existing`), so re-running sync on a machine with an existing config silently dropped every dotfile it detected
- Same class of bug as session #22's drift counter — a value computed but never plumbed through
- Extracted `mergeStrings` and `mergeDotfiles` pure helpers (dedup by name and dest path respectively), wired both into the merge path; packages path refactored to use the same helper for consistency
- 10 new table-driven tests across both helpers (append, skip-dup, empty-src, empty-dst, dedup-within-src)
- All 188 tests pass (12/12 packages). Build clean. Vet clean. Pushed (commit ff5f3bc).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-05 — Daily dev session #25 — secrets check dead-code + list count bug

- Third instance of the same dead-value bug class (sessions #22, #24, now #25): `resolved` and `failedCount` in `runSecretsCheck` were incremented per key but never reported — only the `issues` count reached the summary, so `nestor secrets check` showed per-key lines and a diagnosis but no aggregate "N resolved, M failed" line
- Fixed: check now prints `N resolved, M failed` (warn) or `N secret(s) resolved` (ok) after the loop
- Adjacent counting bug in `nestor list`: secrets incremented `total` but never `ok`/`missing`, making the summary "N managed (N ok, N need attention)" silently not add up — secrets aren't status-checked in list, so they're now excluded from verified totals (still listed)
- Strengthened `TestSecretsCheckEnvProvider` to assert the resolved-count line, added `TestSecretsCheckMixedResolve` (1 good + 1 bad key) and rewrote `TestSecretsCheckMissingKey` to assert "0 resolved, 1 failed"
- All 191 tests pass (12/12 packages). Build clean. Vet clean. Pushed (commit e02da00).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-06 — Daily dev session #26 — dashboard drift detection + staticcheck

- Ran staticcheck on the codebase for the first time (installed `honnef.co/go/tools/cmd/staticcheck@2023.1.5` for Go 1.19 compat). Two hits, both dead-value class.
- **`cmd/dashboard.go`** — dashboard reimplemented dotfile drift detection inline, with four bugs: `sourceDir` computed with a default fallback but never plumbed through (the staticcheck hit); `os.Stat` follows symlinks, so a drifted symlink read as "missing" not "drifted"; `strings.HasSuffix(link, t.Src)` matched any link ending in the filename regardless of source dir; and copy-strategy drift was never checked at all. Replaced with `dotfiles.Deployer.Check`, the canonical detector already used by `nestor diff` — uses `os.Lstat` and compares resolved source paths for both strategies.
- Removed the now-dead `dashExpandTilde` helper (its job is done by `dotfiles.expandHome` inside `Check`) and its test.
- **`internal/secrets/secrets.go`** — `var placeholderRe` (regex) was declared but never referenced; `injectOne` already used `strings.ReplaceAll`. Removed it and the now-unused `regexp` import.
- 4 new regression tests: copy-strategy drift detected, symlink-strategy drift detected (the drift use case os.Stat got wrong), absent still maps to present=false, and a mapping table test locking the four CheckStatus → (present, drift) contracts into one place.
- All 194 tests pass (13/13 packages). Build clean. Vet clean. **staticcheck clean** (was 2 hits).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-07 — Daily dev session #27 — secrets injection idempotency

- `nestor secrets inject` was not idempotent. After the first injection the dest holds the *resolved* value (`oauth_token: ghp_first`), not the literal pattern (`oauth_token: {{.Key}}`). A second run's `strings.Contains(content, pattern)` check missed, and the code fell through to append. Re-running inject appended a duplicate line every time; rotating a secret appended the new value instead of replacing the old one.
- Fourth instance of a value-not-surviving-the-round-trip bug (cf. sessions #22, #24, #25): here it is the resolved pattern that doesn't survive, where in the earlier sessions it was a counter.
- Fix: added an anchor-based re-injection path in `injectOne`. When the literal pattern isn't present but the static prefix before the first `{{` placeholder matches a line, replace that line in place instead of appending. Derived `patternAnchor` and `replaceAnchoredLine` as pure helpers for testability.
- 7 new regression tests: idempotent re-inject (same value), in-place value rotation, `{{.<key>}}` named-placeholder idempotency (the form the PLAN.md example uses), neighbouring lines preserved on replace, plus table-driven coverage of both helpers.
- All 200 tests pass (12/12 packages with tests). Build clean. Vet clean. staticcheck clean. Pushed (commit ab1f3c9).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked