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
