# nestor

Your dev environment, from zero to coding.

One config file, one command, beautiful output. nestor manages your full developer environment lifecycle: packages, dotfiles, secrets, shell config, drift detection, and rollback.

## Why?

- **chezmoi** is powerful but overwhelmingly complex
- **yadm** is just git — no bootstrap, no secrets
- **GNU Stow** is just symlinks — many apps break with them
- **Ansible** is enterprise overkill for personal setup
- **Nix** has an incredible learning cliff
- **Bootstrap scripts** are brittle and rot immediately

nestor does what they all do, but simple and beautiful.

## Install

```bash
go install github.com/jfjrh2014/nestor@latest
```

## Quick Start

```bash
# Create a starter config
nestor init

# Edit it
$EDITOR nestor.yml

# Bootstrap everything
nestor up
```

## Config Reference

```yaml
version: 1

packages:
  common:
    - git
    - neovim
    - ripgrep
  macos:
    - homebrew/cask: visual-studio-code
  linux:
    - snap: code

dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy    # copy | symlink
  templates:
    - src: gitconfig.tmpl
      dest: ~/.gitconfig

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
    - starship

profiles:
  personal:
    packages: [discord, spotify]
  work:
    packages: [slack, zoom]
```

## Commands

| Command | Description |
|---|---|
| `nestor init` | Create a starter nestor.yml |
| `nestor up [--profile <name>]` | Bootstrap packages, dotfiles, secrets, shell from config |
| `nestor diff` | Show drift between config and live state |
| `nestor sync` | Capture current machine state into config |
| `nestor list` | Show all managed items with install status |
| `nestor add [package\|dotfile\|secret] <name>` | Interactively add to config |
| `nestor doctor` | Health check: packages, dotfiles, secrets, provider |
| `nestor edit <template-src>` | Edit a template in `$EDITOR`, preview rendered output |
| `nestor secrets inject` | Resolve and inject secrets into dotfiles |
| `nestor secrets check` | Dry-run: verify provider reachable, every key resolves |
| `nestor profiles` | List all config profiles with package counts |
| `nestor rollback [snapshot-id]` | Restore dotfiles from a snapshot |
| `nestor snapshots` | List available dotfile snapshots |
| `nestor dashboard` | Interactive TUI: packages, dotfiles, secrets at a glance |
| `nestor ci [--quiet]` | Validate config statically; exits non-zero on errors |
| `nestor restore --from <url> [--force] [-o <path>]` | Pull a nestor.yml from a remote URL |
| `nestor import [chezmoi\|yadm\|brewfile] [--dry-run]` | Import from existing tools, auto-detect if none given |
| `nestor version` | Show version, commit, and build date |

`nestor up` auto-snapshots dotfiles before deploying. Snapshots are restored with `nestor rollback`.

## Design Principles

1. **Copy by default** — symlinks break things, copy doesn't
2. **Simple config** — YAML, no new DSL to learn
3. **One command** — `nestor up` does everything
4. **Drift detection** — know when your machine diverges
5. **Beautiful output** — clean TUI, no noise

## License

MIT
