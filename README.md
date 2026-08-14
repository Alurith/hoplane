# hoplane

A protocol-neutral connection directory for fast terminal-based selection.

The project is inspired by [Charmbracelet Wishlist](https://github.com/charmbracelet/wishlist), but a connection is not implicitly SSH. The target architecture is:

```text
discovery → normalized endpoints → picker → connector
```

## Current status

The first vertical slice provides a local declarative catalog with:

- YAML configuration
- `add`, `list`, and `show` commands
- JSON output for automation
- a Bubble Tea picker
- protocol-neutral entries with SSH, RDP, VNC, and custom protocol names

Connection execution and network discovery will be added in later slices.

## Configuration

By default, hoplane reads:

```text
<user config directory>/hoplane/config.yaml
```

The path can be overridden with `--config`.

Example:

```yaml
version: 1

connections:
  - name: office
    protocol: rdp
    host: desktop.example.com
    user: alice
    description: Work desktop
    tags:
      - work

  - name: nas
    protocol: ssh
    host: nas.local
    port: 22
```

Known default ports are SSH `22`, RDP `3389`, and VNC `5900`. Other protocols require an explicit port.

## Commands

```bash
hoplane                         # open the picker
hoplane pick                    # open the picker
hoplane add office --protocol rdp --host desktop.example.com --user alice
hoplane list                    # JSON output
hoplane show office             # JSON output
```

## Development

```bash
go test ./...
go run ./cmd/hoplane
```
