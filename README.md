# hoplane

A protocol-neutral connection directory for fast terminal-based selection.

The project is inspired by [Charmbracelet Wishlist](https://github.com/charmbracelet/wishlist), but a connection is not implicitly SSH. The target architecture is:

```text
static configuration → normalized endpoints → picker → connector
```

## Current status

The first four vertical slices provide:

- YAML configuration and a Bubble Tea picker
- `add`, `list`, `show`, and `connect` commands
- JSON output for automation
- SSH and RDP connection entries
- SSH execution through the local OpenSSH client
- Linux RDP execution through `xfreerdp3`
- standard OpenSSH agent, key, and local configuration support

## Configuration

By default, hoplane reads:

```text
<user config directory>/hoplane/config.yaml
```

The path can be overridden with `--config`.

Example:

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
        client: xfreerdp3
        domain: CONTOSO
        fullscreen: "true"
        ignore_certificate: "true"

  - name: nas
    protocol: ssh
    host: nas.local
    port: 22
    user: alice
```

Hoplane passes standard SSH endpoints to the local `ssh` client. OpenSSH may still use the user's normal agent, keys, and local configuration; Hoplane does not expose separate identity or proxy settings. Known default ports are SSH `22` and RDP `3389`.

RDP connections currently run only on Linux, through `xfreerdp3`. The
`rdp.client` value and `--rdp-client` flag select a logical client ID; when it
is omitted, Linux defaults to `xfreerdp3`. There is no automatic fallback and
client paths, programs, extra arguments, and shell commands are not accepted
from YAML. Windows and macOS retain the RDP model but do not register a client
yet, so attempting to plan an RDP connection there reports that no platform
client is registered.

The `add` command also supports `--rdp-domain`, `--rdp-fullscreen`, and
`--rdp-ignore-certificate`. `rdp.domain` is passed to `xfreerdp3` as
`/d:<domain>`; use the Active Directory domain or the remote computer name for
a local Windows account. The domain is not auto-discovered because it cannot
be reliably inferred from an IP address or hostname. The latter option is
insecure and should only be used for explicitly trusted test environments;
certificate validation is enabled by default. Passwords and secrets are never
written to YAML or passed on the command line. If `xfreerdp3` is not installed,
`connect` returns a required-client error without starting a process. `--dry-run` plans and prints
the invocation without looking up the executable, so it does not require
`xfreerdp3` to be installed.

`list` and `show` emit JSON version 2. The `list` response contains `version`
and `connections`; there is no warnings field because the static configuration
is the only source and its errors are fatal. JSON exposes source provenance and
only the validated, non-secret RDP options (`client`, `domain`, `fullscreen`,
and `ignore_certificate`); a client ID is not filtered merely because it is not
registered on the current platform.

## Commands

```bash
hoplane                         # open the picker
hoplane pick                    # open the picker
hoplane add office --protocol rdp --host desktop.example.com --user alice \
  --rdp-client xfreerdp3 --rdp-domain CONTOSO --rdp-fullscreen \
  --rdp-ignore-certificate
hoplane add nas --protocol ssh --host nas.local --user alice
hoplane list                    # JSON output
hoplane show office             # JSON output
hoplane connect office           # start the RDP client on Linux
hoplane connect office --dry-run # show the RDP command without executing it
hoplane connect nas              # start the SSH client
hoplane connect nas --dry-run   # show the SSH command without executing it
```

In the interactive picker, press `Enter` or `c` to connect. Press `o` to add,
`i` to edit static connections, `y` to duplicate, and `Delete` to remove one
after confirmation. The protocol is selected from SSH or RDP. Optional
form sections can be skipped with `Ctrl+S`. The picker remains open after
successful mutations.

## Development

```bash
go test ./...
go run ./cmd/hoplane
```
