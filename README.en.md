<p align="center"><img src="img/logo.jpg" alt="sshpocket" width="140"></p>

# sshpocket

[![ci](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml/badge.svg)](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/syhgorath/sshpocket)](https://github.com/syhgorath/sshpocket/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/syhgorath/sshpocket/total)](https://github.com/syhgorath/sshpocket/releases)
[![Stars](https://img.shields.io/github/stars/syhgorath/sshpocket?style=flat)](https://github.com/syhgorath/sshpocket/stargazers)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

🇹🇷 [Türkçe](README.md) · 🇬🇧 English

A **single-file** SSH server manager for macOS that you can carry on a USB drive (or run from any folder): a local web UI plus a command line.
Every server is a folder under `servers/<name>/`; the passphrase stays in the macOS Keychain.
Signed **packages** (currently server hardening, time zone/locale setup and installation) can manage the server.

> **Since version 4, sshpocket is rewritten in Go.** The original bash engine is frozen under [`referans/kabuk/`](referans/kabuk) as the **reference**;
> the Go version was verified against it with side-by-side differential tests.

> **Status: raw / early stage.** 🛠️
> This grew out of a personal HomeLab tool. It was reworked together with
> [Claude](https://claude.com/claude-code) and is much better than where it started, but there is
> still plenty to improve: rough edges, missing tests and unwritten features are expected.
> Any problems are down to the project being unfinished, so please be kind 😄 — issues and PRs are
> very welcome. Tested as a client on macOS only (what was tried on the server side: [docs/en/tested-environments.md](docs/en/tested-environments.md)). Read the code before using it on your own servers;
> you use it at your own risk (see [LICENSE](LICENSE)).

> **It does not auto-start.** macOS, Linux and modern Windows do not run scripts when a USB drive is plugged in (a security measure).
> Start it by hand: double-click `start.command` in Finder, or run `./sshpocket`.

## Requirements

| What | Note |
|---|---|
| macOS (Apple Silicon or Intel) | The single binary contains both arm64 and amd64 |
| `ssh`, `ssh-keygen`, `ssh-add`, `ssh-copy-id`, `security`, `osascript`, `diskutil` | All ship with macOS; `./sshpocket doctor` tells you what is missing (it installs nothing) |
| **Python ≥ 3.12** | Only for **packages** (Ansible); `brew install python@3.13`. sshpocket creates the virtual environment it needs |

Signature verification, downloading, archive handling and port probing are built into the binary; `curl`, `tar`, `shasum`, `nc` and `bash` are not needed.

## Quick start

```bash
./sshpocket                         # starts the local web UI (127.0.0.1, tokenised address)
./sshpocket doctor                  # prerequisite check (read-only)
./sshpocket components list         # packages: lock ↔ installed state
./sshpocket components run utility durum ubuntutest   # run a package action from the command line
./sshpocket version
```

The UI covers adding a server (name, address, user, port), key steps (generate, send to the server, add to the agent),
rename, delete, variable editing and package actions (preview → confirm → apply).
The UI binds to `127.0.0.1` only and requires a session cookie.

## Layout (USB / folder)

```
sshpocket          the binary (the universal file from the Release, named like this)
start.command      double-click launcher
release_signers    update trust anchor (signer public key)
index_signers      package-index trust anchor
components.lock    package lock;  components/ installed packages
servers/<name>/    server.env + id_ed25519_<name>   (user data, never committed)
```

## Features

- **One folder per server**, passphrase in the Keychain (never written to disk), adding the key to the agent with the Keychain passphrase (asks to re-save it if it is wrong).
- **Sending the key to the server** (`ssh-copy-id`): does nothing if the key is already authorised; the password is typed only into ssh's own prompt; `PasswordAuthentication no` is deliberately not automatic.
- **Packages:** signed and locked (`components.lock`). Every action passes preview, prerequisite and confirmation gates; the sudo password comes from Ansible's own prompt; logs are `0600` and archived with secrets masked. Details: [docs/en/packages.md](docs/en/packages.md) · hardening (Turkish): [docs/hardening-kullanim.md](docs/hardening-kullanim.md).
- **Updates:** `./sshpocket update` (see below) and `components update | upgrade` for packages.
- **Report:** `components report` shows "what changed and which of my servers is affected" after a package version change.
- **Eject:** `./sshpocket eject` (removes the USB's keys from the agent, then ejects).
- **Tested:** [docs/en/tested-environments.md](docs/en/tested-environments.md) lists what was tried **and what was not**.

- **Importing from `~/.ssh/config`:** `./sshpocket servers import-ssh-config [file]` (private keys are not copied; the `IdentityFile` path is written as `KEY`).
- **Port knocking (client side):** from the UI's server steps or `./sshpocket servers knock <name>`; the sequence stays in the Keychain and "Open in Terminal" sends it automatically before connecting. The server side (knockd) is not part of this tool; classic knocking is not encrypted.
- **Keychain passphrase:** `./sshpocket servers keychain <name>` (save/update/delete, then verifies by unlocking the key).
- **Settings:** `sshpocket.conf` (`STATUS=on|off`, `STATUS_TIMEOUT=1-5`; the `SSHPOCKET_STATUS` environment variable wins). `./sshpocket settings` shows the values, `settings set STATUS_TIMEOUT 3` writes one.

### Not in the Go version (the shell reference had them)

The terminal menu (including fzf) was **deliberately not ported**; server **migration** (`--migrate`, the old `sistem/Modules` layout) was not ported.
Not yet: the 🔒 (knock needed) marker in the status indicator, and the dashboard's "copy ssh command" sending a knock.

## Updating

Never automatic; you start it: `./sshpocket update --check` (look only) and `./sshpocket update` (download → **verify the signature** → confirm the version → install if you agree).
Updates install only a binary downloaded over HTTPS from this repo's GitHub Releases and **signed with the maintainer's key**; nothing is installed if the signature does not verify, if the version the binary reports differs from the tag, or if it would be a downgrade; with no signing key configured, updating is **disabled**.
The swap is atomic, the old binary is backed up under `.update-backup/`; `servers/`, `components/` and your keys are never touched. Details: [go/YAYIN.md](go/YAYIN.md).

### Verifying the first download
`update` verifies signatures itself, but it cannot verify the **first copy** (the file you downloaded from the Releases page): your trust in it rests on GitHub and HTTPS.
You can verify it yourself before running it (OpenSSH ≥ 8.0, ready on macOS). The key's fingerprint is `SHA256:0gjAyj8b7VlmtHIFbwmgAmKjK06sw/a5wdEoE8UECwU`;
confirming it with the maintainer **through another channel** is better than trusting the file in the repo blindly.

```bash
# after downloading sshpocket-X.Y.Z-darwin-universal and its .sig from the Release:
cat > /tmp/sshpocket_signers <<'EOF'
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn
EOF
ssh-keygen -Y verify -f /tmp/sshpocket_signers -I sshpocket-release -n sshpocket-release \
  -s sshpocket-X.Y.Z-darwin-universal.sig < sshpocket-X.Y.Z-darwin-universal      # must print "Good ... signature"
```

## Security model

- **The passphrase is never written to disk.** It lives in the Keychain (`USBMonitor_<PREFIX>_Passphrase`)
  and is passed straight to `ssh-add` via `SSH_ASKPASS`.
- `server.env` holds only IP, user, port (and optionally a key path and knock settings). It must **not** hold secrets; the knock sequence is in the Keychain too.
- **Do not store** TOTP/2FA secrets in this folder; keep them in your phone's authenticator app.
- `servers/`, `id_ed25519*` and logs are kept out of git by `.gitignore`.
- Updates and packages are signed; updating is started by hand (see above). The self-updating binary is bound to the same signing key. Package actions pass preview and confirmation gates before writing to a server.
- IP/user/port values are validated before any command runs (a user name cannot start with `-`).
- The server's host key is asked about on first connection (no automatic acceptance).
- If you carry your private key on a USB drive, encrypt the drive (an encrypted APFS volume).
- To report a vulnerability: [SECURITY.md](SECURITY.md).

## Structure

```
go/                          Go source (the main product): main + internal/{manifest,trust,runner,servers,cli,report,findings,selfupdate,eject,doctor,settings}
go/web/                      embedded web UI
go/YAYIN.md                  release format and release steps (Turkish)
go/scripts/build-release.sh  builds the universal binary and SIGNS it (maintainer tool)
referans/kabuk/              frozen bash engine + Bats tests: the working specification of the Go port
components.lock              package lock file;  components/ installed packages (not committed)
servers/<name>/              server.env, id_ed25519_<name>   (user data)
docs/                        detailed docs (packages, updating, tested environments, standard)
```

## Development

```bash
cd go
go vet ./... && go test ./... -short       # drop -short for the long differential tests
go build -o sshpocket-go .
```

The differential tests run the shell/Python engine in `referans/kabuk/` and Go on the same inputs and compare exit code, output and file tree;
they skip themselves if the shell engine is not found. The shell engine's own tests: `cd referans/kabuk && bats test`.

See [CONTRIBUTING.md](CONTRIBUTING.md) to contribute and [ROADMAP.md](ROADMAP.md) for what is planned.

## Support

sshpocket will stay free and MIT-licensed. If you find it useful you can support it through [GitHub Sponsors](https://github.com/sponsors/syhgorath); stars, bug reports and contributions are just as valuable.

## License

MIT, see [LICENSE](LICENSE).
