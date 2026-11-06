<p align="center"><img src="img/logo.jpg" alt="sshpocket" width="140"></p>

# sshpocket

[![ci](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml/badge.svg)](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/syhgorath/sshpocket)](https://github.com/syhgorath/sshpocket/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/syhgorath/sshpocket/total)](https://github.com/syhgorath/sshpocket/releases)
[![Stars](https://img.shields.io/github/stars/syhgorath/sshpocket?style=flat)](https://github.com/syhgorath/sshpocket/stargazers)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

🇹🇷 [Türkçe](README.md) · 🇬🇧 English

A modular SSH launcher menu for macOS that you can carry on a USB drive (or run from any folder).
Every server is a *module*: drop a module folder in and it shows up in the menu automatically.

> **Status: raw / early stage.** 🛠️
> This grew out of a personal HomeLab tool. It was reworked together with
> [Claude](https://claude.com/claude-code) and is much better than where it started, but there is
> still plenty to improve: rough edges, missing tests and unwritten features are expected.
> Any problems are down to the project being unfinished, so please be kind 😄 — issues and PRs are
> very welcome. Tested on macOS only. Read the code before using it on your own servers;
> you use it at your own risk (see [LICENSE](LICENSE)).

> **It does not auto-start.** macOS, Linux and modern Windows do not run scripts when a USB drive
> is plugged in (by design). Start it manually: `./start.sh`, or double-click `start.command` in Finder.

## Requirements

On a stock macOS **nothing extra needs to be installed**; the tool only uses commands that ship with the system:

| What | Note |
|---|---|
| macOS 10.15 (Catalina) or newer | Signed updates need `ssh-keygen -Y verify` (OpenSSH ≥ 8.0) |
| `bash` 3.2 (`/bin/bash`), `ssh`, `ssh-keygen`, `ssh-add`, `ssh-copy-id`, `security`, `osascript`, `diskutil`, `curl`, `tar`, `shasum`, `nc` | All ship with macOS; `./start.sh --doctor` reports what is missing (it installs nothing) |

Optional:

| What | Used for | Install |
|---|---|---|
| **fzf** | Search and a description preview pane in menus. Without it the built-in arrow-key menu is used. (tried with 0.74.5) | `brew install fzf` |
| **Python ≥ 3.12** | Only for **packages** (e.g. linux-hardening); not needed day to day. macOS's system Python (3.9) is too old | `brew install python@3.13` |

If `fzf` lives in `/opt/homebrew/bin` or `/usr/local/bin` it is also found when started from Finder (`start.command`).

## Quick start

```bash
./start.sh                          # start the menu
```

Everything is available from the main menu: connect to a server, **➕ Add a new server**,
**📥 Import from ~/.ssh/config** and **🛠️ Manage servers** (list/remove/rename).
The same tools work from the command line:

```bash
bash sistem/ModuleGenerator.sh      # create a new server module
bash sistem/ImportSSHConfig.sh      # import hosts from ~/.ssh/config
bash sistem/ModuleManager.sh list   # list modules (also: remove / rename)
```

The generator asks for IP/user/port, can create an ed25519 key and stores the passphrase in the
**macOS Keychain**.

> Note: the interface text and code comments are currently in Turkish; translations are welcome.

## Features

- **Modular:** one folder per server; the menu builds itself.
- **Keychain:** passphrases are never written to disk.
- **Send your key to the server:** `ssh-copy-id` from the menu or the generator.
- **`~/.ssh/config` import:** turns existing hosts into modules. Private keys are **not copied**;
  the `IdentityFile` path is stored as `KEY` in the server's `server.env`.
- **Use an existing key:** set `KEY=/path/to/key` in `servers/<name>/server.env`.
- **Status marks:** the menu shows 🟢 (port open) / 🔴 (unreachable) / ⚪ (not configured), checked in parallel.
  Turn off or tune it under **⚙️ Settings** (`sshpocket.conf`; the env var `SSHPOCKET_STATUS=0|1` takes precedence).
- **Port knocking (client side):** **Knock settings** in the server menu; the sequence (e.g. `7000,8000,9000`) lives in Keychain and is sent automatically
  before connecting; the status mark shows 🔒. The server side (knockd) is not part of this tool; the server admin must set it up. Classic knocking is not encrypted.
- **Module menu:** Connect · Send key to server · **Update details** (IP/user/port/key path) ·
  **Update/delete the Keychain passphrase** (verifies it by unlocking the key after saving). The update options
  stay available even when the settings are broken or the key is missing.
- **Module management:** `ModuleManager.sh list | remove <name> | rename <old> <new>`.
- **Several users/keys for one server:** make one module per identity (e.g. `web_root`, `web_deploy`).
- **Menu:** ↑↓ move · Enter open · `←`/`q` back; typing a number works too; each option shows a short **description**. With **fzf** installed menus open in fzf
  (search + description preview pane). The style is set under ⚙️ Settings (`UI`: `auto` · `fzf` · `arrows` · `numbers`) or with `SSHPOCKET_UI=numbers ./start.sh`;
  use `numbers` if a terminal misbehaves.
- **`./start.sh --doctor`:** prerequisite check (ssh tools, Python range, venv, disk, network); read-only, installs nothing.
- **Packages and updates:** signed, locked (`components.lock`) packages (currently `linux-hardening` and `utility`) live under **📦 Packages and updates** in the main menu; every action goes through preview and confirmation gates, and the sudo password comes from Ansible's own prompt. Commands, platform choice and the mandatory `hardening_ssh_allow_from`: [docs/en/packages.md](docs/en/packages.md) · hardening usage and the **access-network trap**: [docs/hardening-kullanim.md](docs/hardening-kullanim.md) (Turkish).
- **Tested on:** macOS (Intel and Apple Silicon) by hand; macOS (bash 3.2) and Ubuntu 26.04 in CI. What was tried and **what was not**: [docs/en/tested-environments.md](docs/en/tested-environments.md).
- Automated tests (Bats, bash 5.3 on Linux and macOS bash 3.2) + ShellCheck run in CI on every PR.

## Sending the key to the server

**2) Send public key to server** in a module's menu (or the generator's last step) runs `ssh-copy-id`:

- If the key is already authorized it says so and does nothing.
- Otherwise `ssh-copy-id` tries your agent/default keys first and falls back to your **password**.
  You can also point it to a different key file for the login.
- The password is only ever typed into `ssh`'s own prompt; this tool never sees it.
- Setting `PasswordAuthentication no` on the server is **deliberately not automated.** Do it by hand
  after you have confirmed that key login works, or you may lock yourself out.

## Coming from an older version

Servers from before v3 (`sistem/Modules/<name>/`) are migrated **automatically** (`./start.sh --migrate --dry-run`, then `./start.sh --migrate`); old folders are not deleted. Details: [docs/en/migrating.md](docs/en/migrating.md).

## Updating

Never automatic; you start it: `./start.sh --check-update` (only looks) and `./start.sh --update` (download → verify signature → show notes → install if you confirm), or **⬆️ Check for updates / update** in the main menu.
Packages come only from this repo's GitHub Releases over HTTPS and must be **signed with the maintainer's key** (`ssh-keygen -Y verify`); if verification fails nothing is installed, and updating is **disabled** when no signing key is defined. Your servers, settings and keys are kept; on failure the code backup is restored. Details: [docs/en/updating.md](docs/en/updating.md).

### Verifying your first download
`--update` verifies signatures itself, but it cannot verify your **first copy** (the archive you downloaded from the Release page): that trust rests on GitHub and HTTPS.
You can verify the archive yourself before unpacking it (OpenSSH ≥ 8.0, ships with macOS). The key fingerprint is `SHA256:0gjAyj8b7VlmtHIFbwmgAmKjK06sw/a5wdEoE8UECwU`;
confirm it with the maintainer through a **different channel** rather than trusting the file in the repo blindly.

```bash
# after downloading sshpocket-X.Y.Z.tar.gz and sshpocket-X.Y.Z.tar.gz.sig from the Release:
cat > /tmp/sshpocket_signers <<'EOF'
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn
EOF
ssh-keygen -Y verify -f /tmp/sshpocket_signers -I sshpocket-release -n sshpocket-release \
  -s sshpocket-X.Y.Z.tar.gz.sig < sshpocket-X.Y.Z.tar.gz        # must print "Good ... signature"
```

## Security model

- **Passphrases are never written to disk.** They live in the Keychain (`USBMonitor_<PREFIX>_Passphrase`)
  and are handed to `ssh-add` via `SSH_ASKPASS`.
- `server.env` holds only IP, user, port (and optionally a key path, the knock setting) — **never secrets**; the knock sequence also lives in Keychain.
- Do **not** keep TOTP/2FA secrets in this folder; keep them in your phone's authenticator.
- `servers/`, `sshpocket.conf`, `id_ed25519*` and `usb_logs/` are git-ignored.
- IP/user/port are validated before any command runs (the user cannot start with `-`).
- The server's host key is confirmed by you on first connect (no auto-accept).
- If you carry private keys on a USB drive, encrypt the drive (encrypted APFS volume).
- To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Layout

```
start.sh / start.command   entry points
sistem/Main.sh             menu loop
sistem/Autoload.sh         loads the old sistem/Modules/ (until migrated)
sistem/ModuleGenerator.sh  new module (+ key generation, Keychain, ssh-copy-id)
sistem/ImportSSHConfig.sh  import from ~/.ssh/config
sistem/ModuleManager.sh    list / remove / rename
sistem/Helpers/            logging, menu, registry, validation, SSH, eject helpers
servers/<name>/            per server: server.env, id_ed25519_<name>   (old: sistem/Modules/<name>/ → ./start.sh --migrate)
sistem/templates/          server.env.example template
test/                      Bats tests (bats test)
docs/                      detailed docs (packages, updating, tested environments, standard)
img/                       README images (not part of the release archive)
```

## Development

```bash
brew install shellcheck bats-core
shellcheck -x -s bash start.sh start.command sistem/*.sh sistem/Helpers/*.sh
bats test
```

See [CONTRIBUTING.md](CONTRIBUTING.md) to contribute and [ROADMAP.md](ROADMAP.md) for what is planned.

## Support

sshpocket stays free and MIT-licensed. If you find it useful you can support it via [GitHub Sponsors](https://github.com/sponsors/syhgorath); stars, bug reports and contributions are just as welcome.

## License

MIT, see [LICENSE](LICENSE).
