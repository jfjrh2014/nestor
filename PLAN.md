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
### 2026-07-28 — Daily dev session #30 — sync dotfile materialization + merge source bug

- Two bugs in `nestor sync` (the "capture current machine" command):
- **Bug 1 (materialization):** detected dotfiles were written into config as template refs (`.bashrc.tmpl`) but never copied into the source dir. So `nestor up` after `nestor sync` pointed at templates that didn't exist and failed to deploy every scanned dotfile. Added `copyDotfileTemplates` to materialize each detected file as its `.tmpl` counterpart in the source dir.
- **Bug 2 (dead-value, merge path):** `cfg = existing` on the merge path discarded the freshly-computed `sourceDir` when the existing config had empty `Source`. Fifth instance of the dead-value bug class (cf. #22, #24, #25, #27). Fixed: merge now preserves the freshly-computed source dir when the existing config has none.
- Derived `copyDotfileTemplates` and `copyFileSynced` as pure helpers for testability.
- 5 new tests: full-copy happy path, partial-failure (missing source skipped), mode preservation, merge-preserves-source, merge-doesn't-clobber-existing-source.
- All 171 tests pass (12/12 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 02cb5d9).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-29 — Daily dev session #31 — add empty-name panic + input validation

- `nestor add dotfile ""` panicked: `addDotfile` indexed `name[0]` without a length check, crashing with "index out of range [0] with length 0". `addPackage` and `addSecret` had no guard either, and would write empty-keyed entries into the config that `validate()` would later reject on every command that loads it.
- Fixed all three handlers to return an early error before any config load or write. Different bug class than the dead-value family (#22, #24, #25, #27, #30): a missing bounds check rather than a value-not-plumbed-through.
- 3 new regression tests (one per handler). The dotfile test is the key one: without the guard it crashes the test binary with a panic.
- All 174 tests pass (12/12 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 4e16abf).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/
- Next: tag v0.1 once CI unblocked

### 2026-07-31 — Daily dev session #33 — import path plumbing (dead-value class)

- Seventh instance of the dead-value bug class (cf. #22, #24, #25, #27, #30, #32), this time in `cmd/import.go`.
- `importSource` was declared as a package global, written (`importSource = name`), and even read (`_ = importSource`), but its value never reached `importer.NewChezmoi("")` or `importer.NewBrewfile("")` — both were always called with `""`, so the documented `nestor import brewfile ~/Dotfiles/Brewfile` form quietly scanned CWD instead of the named path.
- Worse, `cobra.MaximumNArgs(1)` rejected a second positional arg before the path could even reach the global, so the feature was double-dead: the flag wiring and the arg wiring both ended at nothing.
- Fix: removed the dead global; command now accepts 2 positional args (`[chezmoi|yadm|brewfile] [path]`); extracted `resolveImporter(name, srcPath)` as a pure plumbing function so the name+path → importer mapping is unit-testable; `srcPath` now flows into `NewChezmoi`/`NewBrewfile`. Also fixed a variable shadow on the merge path that wrote the config back to the importer's source path, not the loaded config path.
- Added explicit guard rails: a path passed with `auto` or `yadm` is now rejected with a clear error (they have no configurable source) instead of being silently dropped — the exact failure mode this bug class exploits.
- 5 new tests in `cmd/import_test.go` (unknown source, auto+path rejected, yadm+path rejected, brewfile honours path, chezmoi honours path). Tests prove the path reaches the constructors by asserting the not-found errors, without requiring chezmoi/yadm installed.
- All 180 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 5af2ba5).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-07-30 — Daily dev session #32 — sync effective-source copy bug

- Sixth instance of the dead-value bug class (cf. #22, #24, #25, #27, #30), this time in `nestor sync`.
- When merging detected dotfiles into an existing config that had a *custom* dotfiles source (not the default `~/.config/nestor/dotfiles`), templates were always copied into the freshly-computed default source dir while the merged config kept pointing at the custom source. So `nestor up` after `nestor sync` failed with `CheckSrcMissing` for every newly-detected dotfile — the copy and the config disagreed on where the source was.
- Fix: moved the `copyDotfileTemplates` call to after the merge resolves `cfg`, and compute the effective source from the resolved config (custom if set, else default). Templates now land where the config says they live.
- 1 new regression test (`TestSyncMergeCopiesToEffectiveSource`) that seeds a custom-source config, runs the merge simulation, and asserts the template lands in the custom dir — not the default.
- All 175 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 57e5c4c).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — re-tested the push today, same `refusing to allow an OAuth App to create or update workflow` rejection. Files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-01 — Daily dev session #34 — secrets "no secrets declared" guard bug

- Related to the dead-value family but a distinct shape: a redundant guard condition, not a discarded value.
- `runSecretsInject`, `runSecretsCheck`, and `runDoctor` all used `(cfg.Secrets.Provider == "" || len(cfg.Secrets.Mappings) == 0)` as the "no secrets declared" early-return. The OR short-circuits on an empty provider — but an empty/omitted provider is *valid*: `config.validate()` allows it and `secrets.NewProvider("")` returns the env default. So a config with real mappings but no `provider:` line silently reported "no secrets declared" in all three commands, hiding the user's configured secrets. Worse, `nestor ci` treated the same state as a hard `SeverityError`, so the CLI disagreed with itself.
- Fix: all three sites now key on `len(cfg.Secrets.Mappings) == 0` alone. `doctor` resolves the provider via `NewProvider` so it reports the real provider name (`env`) rather than printing an empty literal. The check command's provider-block was also restructured so a failed `NewProvider` is reported once and the per-key loop still produces a "0 resolved, N failed" summary instead of swallowing the failure.
- 2 new regression tests (`TestSecretsCheckEmptyProviderWithMappings`, `TestSecretsInjectEmptyProviderWithMappings`) that seed a config with mappings and no provider, then assert the mapping key is resolved/reported (not hidden behind "no secrets declared").
- All 182 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit b484d77).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-02 — Daily dev session #35 — `up` secrets guard (session #34 follow-up)

- Direct follow-up to session #34: the empty-provider guard fix was applied to `runSecretsInject`, `runSecretsCheck`, and `runDoctor`, but the same guard survived in `cmd/up.go` Step 5. `nestor up` still used `(len(ms) == 0 && len(pms) == 0) || provider == ""`. So a config with real secret mappings and no `provider:` line ran `secrets inject` fine standalone but got its secrets silently skipped by `nestor up` itself.
- Fix: extracted `hasSecrets(base, profile)` pure helper (keys on mapping count alone, never the provider literal) and guard the step with it. Mirrors the `len(mappings) == 0` fix from #34 but in the orchestrator command.
- 2 new tests: table-driven `TestHasSecrets` (empty/base-only/profile-only/both) and `TestHasSecretsEmptyProviderRegression` (mappings present, provider empty).
- All 184 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit e26d022).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-03 — Daily dev session #36 — `list` empty-provider guard (session #35 follow-up)

- Third location of the same buggy guard from sessions #34/#35: `cmd/list.go` guarded the secrets section with `secTotal == 0 || cfg.Secrets.Provider == ""`. An empty provider is valid (defaults to env), so `nestor list` showed "no secrets declared" for a config with real mappings that other commands handled correctly.
- Fix: drop the `||` clause, key on mapping count alone — identical shape to the fixes applied to `runSecretsInject`, `runSecretsCheck`, `runDoctor`, and `cmd/up.go` in sessions #34–#35. This was the last surviving copy of the guard.
- Refactored `runList` into writer-injected `runListOut` (same pattern as sessions #18/#21) so the bug is unit-testable without os pipe races. Added `cmd/list_test.go` with 2 tests: empty-provider-with-mappings regression and no-secrets-declared non-regression.
- All 186 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 0b8438b).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-04 — Daily dev session #37 — dashboard provider display (final copy)

- Last display-side instance of the empty-provider guard bug family (cf. #34, #35, #36).
- `dashSecretsProvider` in `cmd/dashboard.go` returned `"none"` when `cfg.Secrets.Provider == ""`, but an empty provider is valid and resolves to `env` via `NewProvider("")`. So the dashboard Overview showed "Secrets: N configured (none)" for configs that other commands handled correctly — the CLI disagreed with itself one final time.
- Fix: key on mapping count, not the provider literal. Returns `"none"` only when there are zero mappings (genuinely no secrets), `"env"` when mappings exist but provider is unset, and the literal provider name otherwise.
- Updated `TestDashSecretsProvider` from 2 to 4 cases: added empty-provider-with-mappings (the regression) and set-provider-without-mappings (ensures "none" is still shown when there's truly nothing configured).
- All 188 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 137fa5f).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-05 — Daily dev session #38 — remote error propagation

- Different shape but same family as the dead-value bug class (sessions #22–#37): values computed but discarded, this time *errors* rather than data.
- `nestor remote` and all three subcommands (add, show, remove) used `Run` with `_ =`, and every `runRemote*Out` function printed errors to stderr and returned nil. So a CI script calling `nestor remote add` on a failure path exited 0 — the failure was undetectable via exit code. `push` and `pull` already returned errors properly via `RunE`; `remote` was the lone holdout.
- Fix: converted all three subcommands and the bare `remote` command to `RunE`; rewrote the three `runRemote*Out` functions to return real errors (wrapped with context) instead of swallowing them. No behavior change on success paths — only failures now surface.
- 3 new tests: `TestRemoteAddErrorPropagates` (forces a broken config dir, asserts non-nil error), `TestRemoteShowEmptyStillTrivial` and `TestRemoteRemoveNoOpStillTrivial` (lock in that "nothing to report" is still nil, not a new failure).
- All 189 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 6adcae2).
- ⚠️ CI workflows still blocked (token lacks `workflow` scope) — files ready at .github/workflows/.
- Next: tag v0.1 once CI unblocked

### 2026-08-06 — Daily dev session #39 — restore writer-injection refactor

- Final command in the testability-gap series (cf. #18 push, #21 pull/remote, #36 list): `nestor restore` was the last holdout writing directly to stdout via `fmt.Print*` inside its `RunE` closure, with all logic inlined. Untestable without a binary harness.
- Extracted `runRestoreOut(fromURL, w io.Writer) error` following the same pattern; the `RunE` closure is now a two-liner that reads the flag and delegates.
- Added 4 tests (first tests for `cmd/restore.go`): empty-URL guard, dry-run writes no file, write path creates file, overwrite refused without `--force`. Together they exercise fetch → validate → preview → write/dry-run through a local `httptest.Server`.
- All 193 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 2b74ba2).
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-07 — Daily dev session #40 — doctor writer-injection refactor

- `nestor doctor` (health check) was a notable testability gap: all logic inlined in the `RunE` closure, writing directly to `os.Stdout`, with 5 discovery branches (config, platform, packages, dotfiles, secrets) exercising real system state. Untestable without the os-pipe race pattern that session #18 moved away from.
- Extracted `runDoctorOut(ctx, w io.Writer) error` following the established pattern (#18 push, #21 pull/remote, #36 list, #39 restore). The `RunE` closure is a one-liner delegating to `runDoctor`.
- Added 4 tests covering the key branches: missing config (returns clear error), empty-config baseline (all "no X declared" lines, exits clean), missing dotfiles source dir (flagged as issue, not a hard failure), and missing package detection (declared-but-uninstalled package is reported).
- All 197 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit c914a7a).
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-08 — Daily dev session #41 — edit writer-injection refactor

- `nestor edit` was the last remaining command with all output going to raw stdout via `fmt.Print*` in a `RunE` closure — preview rendered output, render-error messages, and "created new template" notices were all untestable without the os-pipe race pattern.
- Extracted `runEdit(name string, w io.Writer) error` following the established pattern (#18 push, #21 pull/remote, #36 list, #39 restore, #40 doctor). `RunE` is now a one-liner.
- Added 4 new tests (3 prior tests retained): preview rendered output asserted via buffer, render error reported in output (broken template syntax), non-template file skips preview (buffer assertion), new-template created message includes deploy hint.
- All 200 tests pass (12/12 packages with tests). Build clean. Vet clean. staticcheck clean. Pushed (commit b248fdc).
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-09 — Daily dev session #42 — import writer-injection refactor

- `nestor import` was the penultimate command writing directly to stdout via `fmt.Print*` — 9 calls across the preview (packages found, dotfiles found, skipped count), dry-run, nothing-new, and imported paths. All untestable without the os-pipe race pattern.
- Extracted `runImport(name, srcPath string, w io.Writer) error` following the established pattern (#18 push, #21 pull/remote, #36 list, #39 restore, #40 doctor, #41 edit). `RunE` now passes `os.Stdout` explicitly.
- Added 4 new tests (5 prior `resolveImporter` tests retained): dry-run preview output (asserts packages, skipped count, dry-run marker, and that config is untouched), nothing-new-on-second-import (idempotent re-run shows "nothing new to add"), import-and-write (asserts count in output + packages persisted to config file), missing-config-error (load failure surfaces as wrapped error, no output).
- All 204 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 1487031).
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-10 — Daily dev session #43 — init writer-injection refactor (series complete)

- `nestor init` was the last remaining command with output going to raw stdout via `fmt.Print*` in a `RunE` closure — the entire testability-gap series is now complete.
- Extracted `runInit(w io.Writer) error` following the established pattern (#18 push, #21 pull/remote, #36 list, #39 restore, #40 doctor, #41 edit, #42 import). `RunE` is now a one-liner. The starter config content was also lifted to a named const `starterConfig` for clarity.
- Added 2 new tests (first tests for `cmd/init.go`): creates-file-with-deploy-hint (asserts file content + stdout message), refuses-overwrite (pre-places nestor.yml, asserts error + preserved content + no stdout output).
- All 206 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit f02c736).
- **Writer-injection refactor series complete** — every command in the CLI now has its core logic in a testable `runXOut(io.Writer)` function, none dependent on os-pipe races.
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-11 — Daily dev session #44 — rollback writer-injection refactor + first test file

- `nestor rollback` (and `snapshots` / `snapshots prune`) had no dedicated test file and still inlined all logic in `RunE` closures via `ui.New(os.Stdout)` directly — the same testability gap addressed for other commands in sessions #18–#43.
- Extracted `runRollback(id string, w io.Writer) error`, `runSnapshotsList(w io.Writer) error`, and `runSnapshotsPrune(keep int, w io.Writer) error` following the established pattern. Each `RunE` closure is now a one-liner delegating to its `runXOut` counterpart.
- Added `cmd/rollback_test.go` (first test file for the snapshot commands). 7 new tests: rollback-latest restores file content, rollback empty errors, rollback specific snapshot via ID, snapshots list empty, snapshots list populated, prune no-op, prune removes old.
- All 213 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 2d2df04).
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-12 — Daily dev session #45 — profiles + ci writer-injection, pull tests

- Last three `cmd/` files without dedicated test files: `profiles.go`, `ci.go`, and `pull.go` (pull already had writer-injection but no tests).
- Extracted `runProfiles(w io.Writer)` from `profiles.go` and `runCI(w io.Writer)` from `ci.go`, following the established pattern.
- Added `cmd/profiles_test.go` (4 tests): empty config, multi-profile listing (sorted, with detail lines), missing config error, counts in summary.
- Added `cmd/ci_test.go` (5 tests): valid config, invalid config (mappings without provider — passes Load, fails ci.Validate), quiet suppresses success output, quiet still outputs on failure, missing config error.
- Added `cmd/pull_test.go` (1 test): pull without a configured remote fails with a clear error.
- All 281 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 7787980).
- **Every command in the CLI now has a dedicated test file.**
- Next: tag v0.1 once CI workflow-scope token situation is unblocked.

### 2026-08-13 — Daily dev session #46 — packages backend testability

- `internal/packages` had 40.8% coverage: all five backend command invocations (brew, apt, dnf, pacman, snap) called `exec.Command` directly, untestable without hitting real system tools.
- Extracted `cmdRunner` interface + `execRunner` wrapper; backends now take an injected runner at construction.
- 18 new tests in `backends_test.go`: per-backend IsInstalled + Install command construction, brew formula/cask flag selection, InstallAll all-paths/empty/reuse.
- Coverage: 40.8% -> 96.1% of statements in internal/packages. Pushed (commit 68a800a).

### 2026-08-14 — Daily dev session #47 — dotfiles coverage push

- `internal/dotfiles` was the lowest-covered core package at 52.6%.
- 18 new tests: exported `Render` (happy path + parse error), `fallbackCopy` (success, missing src, dest-is-dir), symlink `Check` branches (present, drifted-to-wrong-target), unrenderable-template `Check` -> Drifted, `Status`/`CheckStatus` String tables incl. unknown, `samePath`, nested dest dir creation, absolute Src overriding Deployer.Source, and mkdir/read/write error paths.
- Error-path tests use EISDIR/file-blocker techniques instead of chmod 0500 — they fail correctly whether run as root or a normal user.
- Coverage: 52.6% -> 91.8% of statements. All 259 tests pass. Build clean. Vet clean. staticcheck clean. Pushed (commit 40bda1d).
- Grand total coverage now: ci 88.5%, config 93.9%, dotfiles 91.8%, packages 96.1%, restore 87.1%, shell 74.4%, vcs 70.3%, snapshot 64.2%, platform 63.0%, importer 62.1%, secrets 61.9%, cmd 47.0%.

### 2026-08-15 — Daily dev session #48 — secrets provider testability

- `internal/secrets` at 61.9%: the three CLI-backed providers (1password via `op`, bitwarden via `bw`, vault via `vault`) called `exec.Command` directly — 0% coverage, untestable without real secret managers.
- Extracted `cmdOutput` interface + `execCommand` wrapper: same seam as session #46's `cmdRunner` in packages, adapted to capture command *stdout* (providers need output, backends only needed exit status). Providers construct through the package-level `cmdOut`, tests swap it via `swapCmdOut`.
- 17 new tests: per-provider resolve + failure with exact command-arg assertions (`op read <ref>`, `bw get password <name>`, `vault read -field=<f> <path>`), vault key parsing (`secret/path#field` vs default `value` field), multi-mapping `ResolveAll`, `~` expansion in inject dests, missing-resolved-value error result, mkdir/open error branches (file-blocker and EISDIR, per #47 pattern), Status.String table, expandHome table.
- Coverage: 61.9% → 94.9%. All 271 tests pass (13/13 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 37ef587).
- Coverage standings now: packages 96.1%, secrets 94.9%, config 93.9%, dotfiles 91.8%, ci 88.5%, restore 87.1%, shell 74.4%, vcs 70.3%, snapshot 64.2%, platform 63.0%, importer 62.1%, cmd 47.0%.
- Next: coverage push on importer (62.1%) or platform (63.0%), or a cmd-layer push (47.0%).

### 2026-08-16 — Daily dev session #49 — importer coverage push

- `internal/importer` at 62.1%: root cause was three `os.UserHomeDir()` call sites (NewBrewfile fallback, NewChezmoi default, Yadm.Import) bypassing the existing `osUserHomeDir` test seam in helpers.go — so `Auto()` sat at 0% and NewBrewfile at 37.5% with no way to control home-dir resolution from tests.
- Routed all three call sites through the seam. Made `runCommand` a var (same pattern as #46 cmdRunner / #48 cmdOutput).
- 10 new tests in helpers_test.go: Yadm.Import happy path (asserts `yadm list -a` invocation, `~/` relativization, nested dests, outside-home skip counter), yadm command failure wraps "running yadm list", Auto() priority order — chezmoi found / yadm found / nothing found (each with swapped home, PATH, and clean CWD), Brewfile.Name(), open-after-delete error, `$HOME/.Brewfile` discovery, chezmoi scan-error branch (chmod 0000, skips under root), expandHome table (bare `~`, `~other/`, plain path, failing lookup), MergeResult empty config.
- Moved TestYadmNotInstalled from importer_test.go to helpers_test.go with the other lookup-swap tests.
- Coverage: 62.1% → 92.9%. All 281 tests pass (12/12 packages with tests). Build clean. Vet clean. staticcheck clean. Pushed (commit c4caabf).
- Coverage standings now: packages 96.1%, secrets 94.9%, config 93.9%, importer 92.9%, dotfiles 91.8%, ci 88.5%, restore 87.1%, shell 74.4%, vcs 70.3%, snapshot 64.2%, platform 63.0%, cmd 47.0%.
- Next: platform (63.0%) or snapshot (64.2%), then the cmd layer (47.0%).

### 2026-08-17 — Daily dev session #50 — platform coverage push

- `internal/platform` at 63.0%: `detectOS`/`isWSL`/`detectPackageManager` read `runtime.GOOS`, `/proc/version`, and `exec.LookPath` directly — darwin, WSL, and all error branches unreachable on the CI host.
- Routed all three environment lookups through package vars (`goos`, `lookPath`, `readProcVer`), same seam pattern as #46 (cmdRunner), #48 (cmdOutput), #49 (osUserHomeDir). Also replaced the `exec.Command("cat", "/proc/version")` subprocess with `os.ReadFile` — one fewer fork for every Detect call.
- Rewrote `platform_test.go` around a `swapEnv(t, goos, pathMap, procVer, procErr)` helper. 11 new tests: detectOS table (darwin/linux/wsl/plan9-fallback), isWSL (microsoft string, case-insensitive, plain linux, read-error), detectPackageManager (mac brew, mac no-brew with brew.sh hint, apt/dnf/pacman/snap preference order, no linux manager, unsupported OS), commandExists via swapped PATH, Detect partial-info-on-error.
- Coverage: 63.0% → 96.4% of statements (all five functions at 100%). All 292 tests pass (12/12 packages). Build clean. Vet clean. staticcheck clean. Pushed (commit 60ee117).
- Coverage standings: packages 96.1%, platform 96.4%, secrets 94.9%, config 93.9%, importer 92.9%, dotfiles 91.8%, ci 88.5%, restore 87.1%, shell 74.4%, vcs 70.3%, snapshot 64.2%, cmd 47.0%.
- Next: snapshot (64.2%), then the cmd layer (47.0%).

### 2026-08-18 — Daily dev session #51 — snapshot coverage push

- `internal/snapshot` at 64.2%: the five exported wrappers (Create/List/Restore/Delete/Prune) funneled through `Dir()` -> real `os.UserHomeDir()`, so 0% coverable without touching the real home. Plus a pile of uncovered error branches in createIn/restoreIn/pruneIn/copyFile/writeAtomic.
- Routed `Dir()` and `expandHome` through a `userHomeDir` package-var seam (pattern from #46 cmdRunner, #48 cmdOutput, #49 osUserHomeDir, #50 goos/lookPath).
- 13 new tests: Dir seam + home-error propagation across all six exported entry points, full exported round-trip (Create -> List -> Restore -> Prune -> Delete) against a swapped home, createIn error paths (MkdirAll failure under a file-blocker, same-second dir suffix), restore with a missing manifest-referenced backup, unparseable manifest, listIn/restore read errors, pruneIn mid-failure with partial removed list (0500 parent, root-aware), copyFile missing-src + dir-as-dest, writeAtomic under-file + dir-as-target, sanitizePath windows drive-letter branch, expandHome error branch.
- Also removed a stale `_ = id` dead-assignment in createIn (id is derived from the returned dir path; the discard was legacy from the pre-refactor layout).
- Coverage: 64.2% -> 88.5%. All 299 tests pass. Build clean. Vet clean. staticcheck clean. Pushed (commit pending).
- Coverage standings: packages 96.1%, platform 96.4%, secrets 94.9%, snapshot 88.5%, config 93.9%, importer 92.9%, dotfiles 91.8%, ci 88.5%, restore 87.1%, shell 74.4%, vcs 70.3%, cmd 47.0%.
- Next: the cmd layer (47.0%) is the last low-coverage package.

- Pushed commit 657a803 (snapshot tests + PLAN.md journal entry shipped as one commit).

### 2026-08-19 — Daily dev session #52 — cmd layer: dashboard TUI coverage

- `cmd` at 47.0%: the bubbletea dashboard model (View/Update/Init/listLen + all four render functions) sat at 0% — assumed untestable without a terminal, but the model is a plain value type: construct it loaded, feed it `tea.KeyMsg`/`tea.WindowSizeMsg`/`dashStatusLoadedMsg`/`dashErrMsg` directly.
- 14 new tests in `cmd/dashboard_test.go` (+324 lines): full key-handling matrix (tab cycle, shift+tab wrap to Secrets, 1-4 jumps, up/down with cursor clamping per active tab's listLen, quit keys return tea.Quit), window-size storage, dashStatusLoadedMsg population, dashErrMsg -> error View, listLen per tab, renderTabs pointer marking, all four tab renderers incl. empty states and missing/drifted annotations, overview counters (1/2 installed, 1 ok/drifted/missing, profiles line suppressed when empty), Init returns load command, dashLoadStatus with empty config, dashErrMsg.Error.
- Side coverage: configPath 33% -> 100% (flag / local nestor.yml / home default branches), plus missing-config error paths for runListOut and runPullOut.
- bubbletea v0.25 predates KeyPress — KeyMsg construction is `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}`; msg.String() resolves the name.
- Coverage: cmd 47.0% -> 59.3%. Dashboard model itself: View/render*/listLen/Init at 100%, Update 90.9%. All 313 tests pass (111 in cmd). Build clean. Vet clean. staticcheck clean. Pushed (commit df3694e).
- Coverage standings: packages 96.1%, platform 96.4%, secrets 94.9%, config 93.9%, importer 92.9%, dotfiles 91.8%, ci 88.5%, snapshot 88.5%, cmd 59.3%.
- Next: remaining cmd gaps are thin wrappers (runX -> runXOut glue at 0%, newDashboardCmd cobra wiring) plus runListOut/runPullOut middle paths (39-48%) — diminishing returns; consider calling the coverage campaign done and moving to v0.1 tagging prep.
