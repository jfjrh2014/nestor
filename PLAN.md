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
2026-10-05: vcs.IsUnborn reported every non-repo directory as unborn (rev-parse fails there too). Gated on IsRepo, +2 tests pinning unborn/born/detached/non-repo. 470 tests, gate green. Next: v0.1 (workflow scope still pending).
2026-10-06: add dotfile no longer fossilizes a typo: refuses a missing source file and materializes the .tmpl into the source dir (skip-if-exists, matching sync capture). +2 tests, 472 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-07: restore -o and add dotfile no longer mis-handle ~ paths: restore.Write expands ~/... against the real home (the documented example silently created a literal "~" dir in cwd) and refuses ~user/...; add dotfile refuses ~user/... instead of re-rooting it at the wrong home. +3 tests, 475 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-08: last bare os.WriteFile call sites routed through fsutil.WriteFileSync: nestor init's starter config, edit's new-template starter, and vcs.WriteGitignore (a crash mid-write truncated files config.Load then refused forever). +3 tests, 478 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-09: restore.Fetch silently truncated configs over 10 MB: io.LimitReader(10MB) fills and returns err=nil on oversized bodies, so the tail was cut off before Validate and the durable write. Now reads one extra byte and refuses with a "too large" error; an exactly-at-limit body still lands whole. +1 test, 479 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-10: tilde dests with HOME unset are refused at the write boundaries (deploy, diff check, secret inject, snapshot backup, restore) instead of landing under a literal "~" dir relative to cwd; pathutil.ResolveHome centralizes the check. +6 tests, 485 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-11: remote show/remove without git on PATH reported "no remote configured" (GetRemote swallows the exec error) — remove let the user believe their remote was gone while origin stayed set. All three remote subcommands now gate on HasGit with the shared ErrGitNotFound sentinel. +3 tests, 488 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-12: the restore side of the tilde boundary missed the 2026-10-10 fix: restoreIn copied straight to the manifest's Original, so "~/.bashrc" landed in a literal "~" dir under the cwd (probed). Restore now resolves dests through pathutil.ResolveHome like createIn. +4 tests, 492 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-13: renderTemplate's doc promised builtins (env, home) but only env was registered: any template using the documented {{ home }} failed to parse with `function "home" not defined` on every deploy (probed). Registered home; +3 tests pinning render/deploy/check paths. 495 tests, gate green. Next: v0.1 (workflow scope still pending).
2026-10-13b: Deployer.Check reported a symlink dest under COPY strategy as plain drift (its Readlink-target comparison can't match a foreign link), sending users to an 'up' that refuses symlink dests. New CheckForeignLink verdict; diff/doctor/list/dashboard updated (dashboard now shows the blockage instead of a default-silent green row). +4 tests, 499 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-14: shell.WriteSourceBlock fell through to append on a reversed managed block (end marker before begin: the replace guard never matches, one duplicated block per run — probed), and the durable temp+rename write silently replaced a symlinked rc with a regular file. Reversed shape now refused loudly (same contract as half-removed), writes follow the symlink to the target (link preserved, dangling link refused). +3 tests, 502 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-15: symlink deploy's fallback branch deleted its own success: after the durable fallbackCopy replaced dest, the leftover os.Remove(dest) (predating the durable writer) destroyed the fresh copy while reporting deployed — probed, every deploy on a no-symlink FS (exFAT/network mounts) would lose the dotfile. Removal dropped; fallback keeps the landed copy. +1 test, 503 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-16: durable writes (temp+rename) silently replaced a symlinked dest with a regular file while reporting success — bytes landed nowhere the link pointed, target untouched, link destroyed (probed restore.Write and snapshot restore; shell/secrets/dotfiles each had their own policy, restore+snapshot had none). Guard in fsutil.WriteFileSync covers both primitives and every boundary; shell still follows links by resolving targets first. +4 tests, 507 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-17: restore.Preview hid the profile layer: 'restore --from <team config>' previewed profile names only — profile packages, dotfiles and secret mappings never shown before the user committed the config (family member of the #59-#65 profile sweep). Preview now renders each profile's layered sections in 'up --profile' order, sorted profile names for deterministic output. +3 tests, 510 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-18: add secret fossilized inject targets the injector refuses forever (~user/... paths, any tilde path when the home lookup fails) — the add "succeeded" and every later 'nestor up' died on the mapping. The prompt now applies the same pathutil.ResolveHome rule as the injector and aborts loudly. +4 tests, 514 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-19: secret inject targets that corrupt on every 'up' were accepted: a dest of nestor.yml itself (any form — XDG, --config path, tilde) let injectOne append a rendered "key: value" line to the YAML and brick the next Load, and a dest of a managed dotfile got overwritten silently by the next deploy (templates render with no secret data). Refused at validate (base + profiles), at 'add secret' prompting (XDG + live --config path), and in the engine (config-path seam). +10 tests, 524 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-20: the dotfile side of the inject-collision family was open: a template whose dest is a declared inject target deploys over the injected value on every run without a successful inject (provider down, restore, snapshot), and a template dest of nestor.yml itself rewrites the config that drives it — both accepted (probed). Refused in validate() (base + profile templates vs inject targets across layers), and both 'nestor add' prompts (dotfile dest, secret inject target) apply the same rule so nothing fossilizes. +7 tests, 531 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-21: snapshot manifests under-reported captures: createIn dropped a missing dest with a bare continue, so a restore of that snapshot restored fewer files than the deploy declares with nothing recording the dest legitimately had no backup. Missing dests are now recorded (FileRef with empty Backup); restore skips recorded skips instead of fabricating content, and non-NotExist stat failures abort the capture instead of counting as missing. +2 tests, 533 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-22: the engine side of the template-vs-inject collision family was open: a hand-edited config (or one written by an older nestor that never validated) reached dotfiles.Deploy unrefused, so a template over a declared inject target destroyed the injected value on every run and one over nestor.yml rewrote the config driving the deploy — validate() refuses both, but nothing between validate and the write did. up.go now passes Deployer.ProtectedDests (absolute inject targets + live config path) and Deploy refuses those dests loudly, byte-for-byte intact; status commands only Check(), so their deployers stay guard-free. +4 tests, 537 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-23: the engine self-inject guard only knew the XDG config: `nestor up -c /work/my.yml` with a mapping whose inject target is that same file sailed through validate (IsSelfInject compares XDG only) and the guard (dest == configPathFn() compared raw strings), and every run appended a rendered line to the config driving it. Guard now refuses when the resolved dest (or the raw dest) equals the resolved live config path, wherever --config points. +3 tests, 540 total, gate green. Next: v0.1 (workflow scope still pending).
2026-10-24: WriteSourceBlock ignored its ReadFile error: any rc it couldn't read fell into the append branch with empty content and was regenerated block-only — a mode-000 rc silently loses every user line on the next 'nestor up' (probed; ENOTDIR/dir-at-rc-path only failed later at the rename with cryptic "file exists"). Read errors other than NotExist now refuse with the rc untouched; missing rc stays the first-run append shape. +3 tests, 543 total, gate green. Next: v0.1 (workflow scope still pending).
