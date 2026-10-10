# hoplane

> Keep every remote destination one keystroke away.

Hoplane is a fast, focused terminal companion for SSH and RDP. It turns your
connection list into an organized, searchable workspace, so you can reach the
right server or desktop in seconds.

It uses the native clients you already trust, stores optional login passwords
in the system keyring, and does not replace OpenSSH or your RDP application. It
simply makes them easier to access.

## Features

- One organized catalog for SSH and RDP connections
- Fast terminal-based connection picker
- Add, edit, duplicate, and remove connections interactively
- JSON output for scripts and automation
- Dry-run mode to preview connection commands
- Custom labels, descriptions, tags, users, and ports
- Native SSH and RDP client integration
- Optional password storage in the system keyring without command-line exposure
- Verified self-update from GitHub Releases
- Local YAML configuration
- OSC 7501 Program Status Protocol support

## Program status

Hoplane reports its connection state using the [OSC 7501 Program Status Protocol](https://www.superlogical.com/rex/docs/build/program-status). Compatible terminals can use these reports to show:

- `idle`: the connection picker is open without an active connection
- `working`: a connection is active
- `done`: the connection closed normally
- `error`: the connection closed with an error

Reports identify the application as `hoplane` and include the connection name
as the record title. Terminals that do not support OSC 7501 ignore these
reports.

## Supported platforms

Linux is currently supported, including RDP connections through
`xfreerdp3`. On other platforms the default password backend is unavailable
and password automation fails closed; native keyring backends are future work.

## Requirements

- Go `1.26.9` to build Hoplane
- `ssh` available in `PATH` for SSH connections
- `xfreerdp3` available in `PATH` for RDP connections on Linux
- `/usr/bin/secret-tool` and an unlocked Secret Service keyring for stored passwords

## Installation

### From a GitHub release

Download the matching archive and `checksums.txt` from the
[latest release](https://github.com/Alurith/hoplane/releases). On Linux:

```bash
cd ~/Downloads
VERSION=0.3.0
ARCHIVE="hoplane_${VERSION}_linux_amd64.tar.gz"

grep "  ${ARCHIVE}$" checksums.txt | sha256sum -c -

tmp=$(mktemp -d)
tar -xzf "$ARCHIVE" -C "$tmp"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$tmp/hoplane" "$HOME/.local/bin/hoplane"
rm -rf "$tmp"
```

Use the `arm64` archive on Linux ARM64. Keep `~/.local/bin` in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Installing there keeps the directory writable for `hoplane update`.

### From source

Clone the repository and install the binary:

```bash
git clone https://github.com/Alurith/hoplane.git
cd hoplane

go install ./cmd/hoplane
```

Alternatively, build a local binary:

```bash
go build -o bin/hoplane ./cmd/hoplane
```

Source builds are development builds and cannot self-update.

## Releases

Releases are published automatically by GoReleaser when a semantic-version tag
is pushed. The release workflow runs the quality checks first, then builds and
uploads the archives and `checksums.txt` to GitHub Releases.

```bash
git tag v0.3.0
git push origin v0.3.0
```

Tags must use the `vMAJOR.MINOR.PATCH` format. The generated archives use the
`hoplane_<version>_<os>_<arch>` naming convention required by self-update.

A released binary can update itself from GitHub Releases after verifying the
published SHA-256 checksum:

```bash
hoplane update
```

The executable directory must be writable. Development builds from `go run`,
`go build`, or `go install` report that self-update is unavailable.

## Quick start

Add an SSH connection:

```bash
hoplane add nas \
  --protocol ssh \
  --host nas.local \
  --user alice
```

List your connections:

```bash
hoplane list
```

Preview the connection command:

```bash
hoplane connect nas --dry-run
```

Connect:

```bash
hoplane connect nas
```

Open the interactive picker:

```bash
hoplane
```

Store a password without putting it in shell history or process arguments:

```bash
hoplane secret set office
hoplane secret status office
hoplane secret clear office
```

`secret set` reads a hidden password from a TTY, or one line from stdin when
input is redirected. The password is not accepted as a flag, argv value,
environment variable, YAML field or command output. Linux requires
`/usr/bin/secret-tool` and an unlocked Secret Service keyring; an unavailable
backend is an error, not a plaintext fallback.

Add an RDP connection:

```bash
hoplane add office \
  --protocol rdp \
  --host desktop.example.com \
  --user alice \
  --rdp-domain CONTOSO \
  --rdp-fullscreen
```

## Commands

| Command | Description |
| --- | --- |
| `hoplane` | Open the connection picker |
| `hoplane pick` | Open the connection picker |
| `hoplane add <name>` | Add a connection and print it as JSON |
| `hoplane list` | Print all connections as JSON |
| `hoplane show <name>` | Print one connection as JSON |
| `hoplane connect <name>` | Launch the configured client |
| `hoplane secret set/clear/status <name>` | Manage the login password in the system keyring |
| `hoplane update` | Update to the latest verified release |
| `hoplane --version` | Print the installed version |

Use a custom configuration file with:

```bash
hoplane --config ./config.yaml list
```

The `--config` flag also has the short form `-c`.

### `add` options

```text
--protocol <ssh|rdp>       Connection protocol
--host <host>              Connection host
--port <port>              Connection port
--user <user>              Optional username
--description <text>       Optional description
--tag <tag>                Connection tag; may be repeated

--rdp-domain <domain>      RDP authentication domain
--rdp-fullscreen           Start RDP in fullscreen
--rdp-ignore-certificate  Ignore the RDP server certificate
```

When omitted, the port defaults to:

- SSH: `22`
- RDP: `3389`

## Interactive picker

The terminal picker lets you quickly browse and manage your connections.

| Key | Action |
| --- | --- |
| `Enter` / `c` | Connect |
| `o` | Add a connection |
| `i` | Edit the selected connection |
| `y` | Duplicate the selected connection |
| `Delete` / `Backspace` | Remove the selected connection |
| `Ctrl+S` | Skip an optional form section |
| `Esc` | Cancel the current action |
| `Ctrl+C` | Exit |

## Configuration

By default, Hoplane reads:

```text
<user config directory>/hoplane/config.yaml
```

Example configuration:

```yaml
version: 3

# Version 3 requires an immutable id on every connection.
connections:
  - id: 5855fc97344cf34c1b1fb886f267d2f2
    name: office
    protocol: rdp
    host: desktop.example.com
    user: alice
    description: Work desktop
    tags:
      - work
    options:
      rdp:
        domain: CONTOSO
        fullscreen: "true"

  - id: 6855fc97344cf34c1b1fb886f267d2f2
    name: nas
    protocol: ssh
    host: nas.local
    user: alice
    tags:
      - home
      - storage
```

### Connection fields

| Field | Description |
| --- | --- |
| `id` | Stable opaque connection identifier; used for secrets |
| `name` | Connection name |
| `protocol` | `ssh` or `rdp` |
| `host` | Hostname or IP address |
| `port` | Optional port |
| `user` | Optional username |
| `description` | Optional description |
| `tags` | Optional list of tags |
| `options` | Protocol-specific options |

### RDP options

RDP options are stored under `options.rdp`:

| Option | Description |
| --- | --- |
| `domain` | Authentication domain |
| `fullscreen` | Start in fullscreen mode |
| `ignore_certificate` | Disable certificate validation |

On Linux, RDP connections use the built-in `xfreerdp3` integration.

## SSH and RDP integration

### SSH

Hoplane launches the local `ssh` client:

```text
ssh -p 22 -l alice -- nas.local
```

Your existing OpenSSH configuration, agent, and keys remain available. An
optional login password can be stored in the system keyring. When present,
Hoplane supplies it through a one-shot `SSH_ASKPASS` helper using a
short-lived owner-only temporary file; it is never an argv element or
environment value. SSH passwords are limited to 1023
bytes and the helper accepts only OpenSSH's exact
`<user>@<host>'s password:` prompt, with at most one trailing ASCII space.
Password authentication is restricted to OpenSSH's normal password method and
`NumberOfPasswordPrompts=1`. Key passphrases, MFA, OTP and
keyboard-interactive authentication remain the responsibility of OpenSSH and
your agent.

### RDP

On Linux, Hoplane launches `xfreerdp3`:

```text
xfreerdp3 /v:desktop.example.com:3389 /u:alice /d:CONTOSO /f
```

An optional RDP login password is stored in the system keyring and passed to
FreeRDP through `/args-from:fd:3`. The password uses its own 1 MiB limit and
is not placed in the process arguments, environment or configuration file.
`connect --dry-run` never reads the keyring and never includes a password.

Certificate validation can be disabled explicitly with:

```bash
--rdp-ignore-certificate
```

Use this option only for trusted test environments.

## Password security

The keyring is the only persistent password store. Hoplane rejects password
values containing NUL, carriage-return or newline characters because both the
FreeRDP argument channel and SSH askpass are line-oriented. SSH passwords are
limited to 1023 bytes; RDP passwords have a separate 1 MiB limit. Secrets are
cleared from application buffers on a best-effort basis; Linux core dumps are
disabled and dumpability is reduced, but Go/runtime and client copies cannot be
physically guaranteed away.

This protects against process-list inspection, shell history, configuration
backups and ordinary application logs. It does not protect against root,
malware or ptrace access from the same user, a compromised keyring, or copies
already made by libraries and clients.

Connection IDs are opaque and immutable in YAML and JSON. Renaming a connection
does not change its keyring entry; duplicating a connection creates a new ID and
does not copy its password. Deleting a connection also deletes its keyring
entry. Hoplane rejects older configuration versions; it does not perform an
automatic v2 migration.

## Dry run

Preview a connection without launching the client:

```bash
hoplane connect nas --dry-run
```

Example output:

```text
dry-run: connection "nas" would execute ssh -p 22 -l alice -- nas.local
```

## JSON output

The `add`, `list`, and `show` commands emit indented JSON version `2`.

Example:

```json
{
  "version": 2,
  "connection": {
    "id": "6855fc97344cf34c1b1fb886f267d2f2",
    "name": "nas",
    "protocol": "ssh",
    "host": "nas.local",
    "port": 22,
    "user": "alice",
    "source": {
      "name": "static",
      "id": "/home/alice/.config/hoplane/config.yaml"
    }
  }
}
```

RDP connections also expose their validated options and certificate security
status. JSON contains the opaque connection ID and public connection metadata
only; it never performs a keyring lookup or contains a password.

## Development

Run the test suite:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

Run Hoplane from source:

```bash
go run ./cmd/hoplane
```

Useful `just` recipes:

```bash
just check
just test
just vet
just vulncheck
just build
just run
```

## Project structure

```text
cmd/hoplane/       Application entrypoint
internal/cli/      CLI commands
internal/config/   YAML configuration
internal/catalog/  Connection catalog
internal/tui/      Terminal interface
internal/connector SSH/RDP integration
internal/output/   JSON output
internal/domain/   Core domain model
```

## License

Hoplane is released under the [MIT License](LICENSE).
