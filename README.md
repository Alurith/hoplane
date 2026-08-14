# hoplane

A protocol-neutral connection directory for fast terminal-based selection.

The project is inspired by [Charmbracelet Wishlist](https://github.com/charmbracelet/wishlist), but a connection is not implicitly SSH. The target architecture is:

```text
discovery → normalized endpoints → picker → connector
```

## Current status

The first four vertical slices provide:

- YAML configuration and a Bubble Tea picker
- `add`, `list`, `show`, and `connect` commands
- JSON output for automation
- protocol-neutral entries with SSH, RDP, VNC, and custom protocol names
- SSH execution through the local OpenSSH client
- Linux RDP execution through `xfreerdp`
- SSH identity files, proxy jumps, and local agent/key support
- discovery of concrete aliases from `~/.ssh/config`

## Configuration

By default, hoplane reads:

```text
<user config directory>/hoplane/config.yaml
```

The path can be overridden with `--config`. The SSH config path can be overridden independently with `--ssh-config`.

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
    options:
      rdp:
        client: xfreerdp
        fullscreen: "true"
        ignore_certificate: "true"

  - name: nas
    protocol: ssh
    host: nas.local
    port: 22
    options:
      ssh:
        identity_file: ~/.ssh/id_ed25519
        proxy_jump: bastion
```

SSH aliases from `~/.ssh/config` are available automatically. Use `--ssh-config` to select another OpenSSH config file. The connector delegates agent authentication, local key loading, and config semantics to the installed `ssh` client.

Known default ports are SSH `22`, RDP `3389`, and VNC `5900`. Other protocols require an explicit port.

RDP connections run on Linux through `xfreerdp`. The `add` command supports
`--rdp-client`, `--rdp-fullscreen`, and `--rdp-ignore-certificate`; these flags
are persisted in the `rdp` options namespace. Passwords and secrets are never
written to YAML or passed on the command line. If `xfreerdp` is not installed,
`connect` reports a required-client error without starting a process. `--dry-run`
only prints the planned invocation, so it does not require `xfreerdp` to be installed.

## Commands

```bash
hoplane                         # open the picker
hoplane pick                    # open the picker
hoplane add office --protocol rdp --host desktop.example.com --user alice \
  --rdp-client xfreerdp --rdp-fullscreen --rdp-ignore-certificate
hoplane add nas --protocol ssh --host nas.local --identity-file ~/.ssh/id_ed25519 --proxy-jump bastion
hoplane list                    # JSON output
hoplane show office             # JSON output
hoplane connect office           # start xfreerdp on Linux
hoplane connect office --dry-run # show the xfreerdp command without executing it
hoplane connect nas              # start the SSH client
hoplane connect nas --dry-run   # show the SSH command without executing it
```

## Development

```bash
go test ./...
go run ./cmd/hoplane
```
