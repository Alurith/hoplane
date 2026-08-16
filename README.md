# hoplane

> Keep every remote destination one keystroke away.

Hoplane is a fast, focused terminal companion for SSH and RDP. It turns your
connection list into an organized, searchable workspace, so you can reach the
right server or desktop in seconds.

It uses the native clients you already trust, never stores passwords or
credentials, and does not replace OpenSSH or your RDP application. It simply
makes them easier to access.

## Features

- One organized catalog for SSH and RDP connections
- Fast terminal-based connection picker
- Add, edit, duplicate, and remove connections interactively
- JSON output for scripts and automation
- Dry-run mode to preview connection commands
- Custom labels, descriptions, tags, users, and ports
- Native SSH and RDP client integration
- Verified self-update from GitHub Releases
- Local YAML configuration

## Supported platforms

Linux is currently supported, including RDP connections through
`xfreerdp3`. Support for additional platforms is in progress.

## Requirements

- Go `1.25.13` to build Hoplane
- `ssh` available in `PATH` for SSH connections
- `xfreerdp3` available in `PATH` for RDP connections on Linux

## Installation

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

## Releases

Releases are published automatically by GoReleaser when a semantic-version tag
is pushed. The release workflow runs the quality checks first, then builds and
uploads the archives and `checksums.txt` to GitHub Releases.

```bash
git tag v0.1.0
git push origin v0.1.0
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

--rdp-client <id>          RDP client identifier
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
version: 2

connections:
  - name: office
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

  - name: nas
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
| `client` | RDP client identifier |
| `domain` | Authentication domain |
| `fullscreen` | Start in fullscreen mode |
| `ignore_certificate` | Disable certificate validation |

On Linux, omitting `client` uses the built-in `xfreerdp3` integration.

## SSH and RDP integration

### SSH

Hoplane launches the local `ssh` client:

```text
ssh -p 22 -l alice -- nas.local
```

Your existing OpenSSH configuration, agent, and keys remain available. Hoplane
does not manage SSH credentials or replace the SSH client.

### RDP

On Linux, Hoplane launches `xfreerdp3`:

```text
xfreerdp3 /v:desktop.example.com:3389 /u:alice /d:CONTOSO /f
```

Hoplane does not store passwords or pass them as command-line arguments.
Authentication remains handled by the local RDP client.

Certificate validation can be disabled explicitly with:

```bash
--rdp-ignore-certificate
```

Use this option only for trusted test environments.

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
status.

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
