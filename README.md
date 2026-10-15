# Spock

A human-friendly CLI and TUI for managing SSH keys, connections, and sessions.

```
spock ssh to worker-1
spock key create --name deploy --type ed25519
spock session mirror calm-tiger
```

## Features

- **SSH Connections** — save, edit, connect, test, import from `~/.ssh/config`
- **SSH Keys** — generate, import, rename, push to remote hosts
- **Sessions** — track active sessions, mirror (read-only watch), session logging
- **Node Describe** — auto-detect CPU, memory, disk, GPU, sysctl, systemd services
- **TUI Dashboard** — full-screen terminal UI when you run `spock` with no args
- **OS Keychain** — passwords stored securely in macOS Keychain / Windows Credential Manager / Linux Secret Service
- **Multi-user** — select which user to SSH as from a dropdown, remembers per connection
- **Human-readable session IDs** — `calm-tiger`, `brave-owl` instead of hex

## Install

### From source

Requires Go 1.22+.

```bash
git clone https://github.com/muhammadjihad/spock.git
cd spock
go build -o spock .
```

Move the binary to your PATH:

```bash
sudo mv spock /usr/local/bin/
```

### From GitHub Releases

Download the pre-built binary for your platform from the [Releases](https://github.com/muhammadjihad/spock/releases) page.

```bash
# macOS (Apple Silicon)
curl -L https://github.com/muhammadjihad/spock/releases/latest/download/spock-darwin-arm64.tar.gz | tar xz
sudo mv spock /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/muhammadjihad/spock/releases/latest/download/spock-darwin-amd64.tar.gz | tar xz
sudo mv spock /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/muhammadjihad/spock/releases/latest/download/spock-linux-amd64.tar.gz | tar xz
sudo mv spock /usr/local/bin/
```

## Quick Start

```bash
# Launch the TUI dashboard
spock

# Or use CLI commands directly:

# Add a connection
spock ssh add worker-1 --host 10.0.0.5 --user root --password

# Connect
spock ssh to worker-1

# Generate a key
spock key create --name deploy --type ed25519

# Push key to remote host
spock key push deploy worker-1

# Describe a node (CPU, RAM, disk, GPU, services)
spock ssh describe worker-1

# Import from ~/.ssh/config
spock ssh import
```

## Commands

### SSH Connections

```
spock ssh                        List all saved connections
spock ssh add <name>             Add a connection (--host, --user, --port, --key, --password)
spock ssh to <name>              Connect (--user to override)
spock ssh edit <name>            Edit a connection
spock ssh remove <name>          Remove a connection
spock ssh show <name>            Show connection details
spock ssh test <name>            Test connectivity
spock ssh describe <name>        Show remote system info (CPU, GPU, RAM, disk, services)
spock ssh copy <src> <dst>       Duplicate a connection profile
spock ssh import                 Import from ~/.ssh/config
spock ssh export <name>          Print as SSH config block
```

### SSH Keys

```
spock key                        List all managed keys
spock key create                 Generate a key (--name, --type, --bits, --comment, --passphrase)
spock key show <name>            Show fingerprint, public key, metadata
spock key remove <name>          Delete a key pair
spock key rename <old> <new>     Rename a key
spock key copy <name>            Copy public key to clipboard
spock key push <key> <conn>      Install public key on remote host (ssh-copy-id)
spock key import <path>          Import an existing key (--name)
```

### Sessions

```
spock session                    List active sessions
spock session mirror <id>        Watch a session in real-time (read-only)
spock session kill <id>          Terminate a session
spock session log <id>           Show session transcript
spock session clean              Remove stale session records
```

## TUI Dashboard

Run `spock` with no arguments to launch the full-screen TUI.

**Navigation:**
- `Tab` — switch focus between sidebar and content
- `↑↓` / `j` `k` — navigate
- `Enter` — connect (opens new Terminal.app window) / mirror session / show key
- `i` — describe selected node (CPU, GPU, RAM, disk, services)
- `n` — new connection or key
- `e` — edit selected connection
- `d` — delete with confirmation
- `c` — copy public key to clipboard (keys view)
- `q` — quit

## Data Storage

All data is stored in `~/.spock/`:

```
~/.spock/
├── config.json          # global settings
├── connections.json     # saved SSH connections + cached node info
├── keys/                # managed SSH key pairs
├── sessions/
│   ├── active.json      # active session registry
│   └── sockets/         # Unix sockets for session mirroring
└── logs/                # session transcripts
```

Passwords are stored in the OS keychain, never on disk.

## Building

```bash
# Build for current platform
go build -o spock .

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o spock-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o spock-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o spock-windows-amd64.exe .

# Run tests
go test ./...
```

## License

MIT
