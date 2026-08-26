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

Or build from source:

```bash
git clone https://github.com/jfjrh2014/nestor.git
cd nestor
make install
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
| `nestor diff [--profile <name>]` | Show drift between config and live state (profile-aware) |
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
| `nestor push [--remote <url>]` | Commit and push your config to a git remote |
| `nestor pull [--remote <url>]` | Pull config changes from a git remote |
| `nestor remote add <url>` | Set the git remote for config sync |
| `nestor remote show` | Show the configured git remote URL |
| `nestor remote remove` | Remove the configured git remote |

`nestor up` auto-snapshots dotfiles before deploying. Snapshots are restored with `nestor rollback`.

### Multi-machine sync

Version-control your config across machines with git:

```bash
# On machine A
nestor remote add https://github.com/you/dotfiles.git
nestor push

# On machine B
nestor pull --remote https://github.com/you/dotfiles.git
nestor up
```

## Shell Completion

nestor generates completion scripts for bash, zsh, fish, and powershell:

```bash
# bash
nestor completion bash > /etc/bash_completion.d/nestor

# zsh
nestor completion zsh > "${fpath[1]}/_nestor"

# fish
nestor completion fish > ~/.config/fish/completions/nestor.fish

# powershell
nestor completion powershell | Out-String | Invoke-Expression
```

## Development

```bash
make test      # run tests
make vet       # go vet
make lint      # golangci-lint (install from https://golangci-lint.run)
make build     # compile with version info
make release   # cross-compile binaries + checksums into dist/
```

## Design Principles

1. **Copy by default** — symlinks break things, copy doesn't
2. **Simple config** — YAML, no new DSL to learn
3. **One command** — `nestor up` does everything
4. **Drift detection** — know when your machine diverges
5. **Beautiful output** — clean TUI, no noise

## License

MIT
