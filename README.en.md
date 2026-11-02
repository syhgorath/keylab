# sshpocket

[![ci](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml/badge.svg)](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml)
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

## Tested environments

| Environment | Status |
|---|---|
| **Intel Mac (x86_64), macOS 15.7.9, bash 3.2.57, OpenSSH 9.9p2** | ✅ Developed and manually tried |
| **Apple Silicon (arm64, Apple M5 Pro), macOS 27.0.1 and 27.2, bash 3.2.57, OpenSSH 10.3p1 / 10.5p1** | ✅ Manually tried: `--doctor`, `ssh-add` with the passphrase from the Keychain, `--components sync` and `run` (`audit` → `preview` → `apply` → second `preview` on an Ubuntu 26.04.1 VM, including password sudo with sudo-rs; `apply` and `audit` on a Proxmox VE 9.2.21 VM; these on 27.0.1. On macOS 27.2 only `--doctor` was re-run). ⚠️ Not tried by hand: opening a Terminal window, double-clicking `start.command`, the arrow-key/fzf look of the new `🛡️ Packages` menu, knocking against a real server |
| **Apple Silicon (arm64), macOS 26.6.2, bash 3.2.57, OpenSSH 10.3p1 (GitHub Actions)** | ✅ **Automated tests** (Bats + ShellCheck, on every PR). ⚠️ Not tried by hand: the Keychain permission dialog, opening Terminal, double-clicking `start.command` |
| Ubuntu 24.04 (CI only: ShellCheck + Bats tests) | ✅ automated |
| Running on Linux (no Keychain/`osascript`/`diskutil`) | ❌ Not supported |
| Target server: Ubuntu 26.04.1 (linux-hardening) | ✅ fresh install on a real VM: Lynis 58 → 76, second run `changed=0`, password sudo-rs |
| Target server: Proxmox VE 9 (Debian 13, inside a VM; linux-hardening `proxmox-host` platform) | ✅ `apply` and `audit` (Lynis 82–84), `pve_firewall_preview` / `pve_firewall_apply` (rollback safety net armed → new ssh connection and web UI verified → disarmed; second preview `changed=0`), web UI and container creation fine. ⚠️ Clustered Proxmox and real hardware not tried; Raspberry Pi not tried |

If you hit a problem on Apple Silicon or a newer macOS, please open an issue (with the macOS version, `uname -m` and `bash --version`).

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
- **Packages (infrastructure):** safe install of signed, locked (`components.lock`) packages: `./start.sh --components list|sync|check-new|prune`; clean up old version folders with `./start.sh --components prune [--dry-run] [--yes]` (the active, the previous and the locked version are kept); it is also offered after an update and available from **Settings › Eski paket sürümlerini temizle** (clean up old package versions); run an installed package's action: `./start.sh --components run <package> <action> <server>` (connects to the server; preview/confirmation gates apply).
  `linux-hardening 0.12.0` is in the lock. With sshpocket 3.6.20 it was tried on real VMs (Ubuntu 26.04.1 and a single-node Proxmox VE 9): the server-type and allowed-network question (when adding a server and on first open), ⛔ when `hardening_ssh_allow_from` is empty, `apply` and a second `preview` (`changed=0`), `hardening_pve_apply` being passed only for that run on Proxmox (and not on Ubuntu), `pve_firewall_*` and the fail2ban settings (`ignoreip` empty + yours, `bantime` 10 min; **an active ban was not tried**). (The 0.9.0 area switches (`hardening_<area>_enabled`) and the permanent-removal actions (`kaldir_onizle`, `kaldir`, `kaldir_geri_yukle`) were tried on real Ubuntu 26.04 and Proxmox VE 9 VMs in earlier rounds; the detailed results below are for 0.8.4; every action tried through sshpocket on the Docker LAB; on real VMs, Ubuntu 26.04 and a single-node Proxmox VE 9, the `audit` → `preview` → `apply` → second `preview` (`changed=0`) cycle was tried, Lynis about 61→83 on Ubuntu and 84 on Proxmox. Also verified on 0.8.4: platform-based menu filtering (`pve_firewall_*` and the Proxmox variables are hidden on Ubuntu), the six per-server switches (`false` writes nothing and does not delete what was already written), the legal banner surviving a reboot on Proxmox, `hardening_pve_web_exclusive` (a same-subnet client that is not on the list can no longer reach 8006 and 3128) and an extra firewall port. **Known limits:** `kernel_modules_disabled` is deliberately refused on Proxmox; `false` does not undo a change that was already applied; **an active fail2ban ban, rejection from an unlisted network, Proxmox with guest VMs/CTs, a Docker VM, Raspberry Pi, an HA cluster and some Ubuntu options (`apt_upgrade`, `restrict_compilers` with a real `gcc`, `auditd_immutable`, `aide_audit_tools`) were not tried**). Usage and the **access-network trap**: [docs/hardening-kullanim.md](docs/hardening-kullanim.md) (Turkish); standard: [docs/package-standard.md](docs/package-standard.md).
- **Server type and allowed network:** if a package declares a platform list (linux-hardening 0.12.0: Ubuntu/Debian, Proxmox VE, Raspberry Pi), the server type is asked when adding a server and, for older servers, the first time the package menu opens ("Not now" writes nothing). **`hardening_ssh_allow_from` is required:** while empty, `preview`, `apply` and `pve_firewall_*` stop with ⛔ before any ssh (`audit` and `rollback_*` are not affected); suggestions come from the address the server sees you at and are never written silently. On Proxmox, `apply` passes the selected platform's `hardening_pve_apply=true` for that run only (the warning is shown before the server-name confirmation and nothing is written to your `.vars` file). Details: [docs/sunucu-ekleme-platform-secimi.md](docs/sunucu-ekleme-platform-secimi.md) and [docs/hardening-kullanim.md](docs/hardening-kullanim.md) (Turkish).
- Automated tests (Bats, bash 5 and macOS bash 3.2) + ShellCheck run in CI on every PR.

## Sending the key to the server

**2) Send public key to server** in a module's menu (or the generator's last step) runs `ssh-copy-id`:

- If the key is already authorized it says so and does nothing.
- Otherwise `ssh-copy-id` tries your agent/default keys first and falls back to your **password**.
  You can also point it to a different key file for the login.
- The password is only ever typed into `ssh`'s own prompt; this tool never sees it.
- Setting `PasswordAuthentication no` on the server is **deliberately not automated.** Do it by hand
  after you have confirmed that key login works, or you may lock yourself out.

## Migrating from the old layout (pre-v3 → `servers/`)

Your old `sistem/Modules/<name>/` servers are migrated **automatically**; no export/import needed.

```bash
./start.sh --migrate --dry-run   # see what would happen (changes nothing)
./start.sh --migrate             # asks for confirmation, then migrates
```

Keys are copied and verified, old folders are **not deleted** (`servers/.migrated-<date>/`). Keychain passphrases stay valid.
Once you have verified everything works, delete that folder yourself (it holds copies of the keys).

## Updating

Never automatic; you start it:

```bash
./start.sh --check-update   # only looks (exit code 10 if a newer version exists)
./start.sh --update         # download → verify signature → show notes → install if you confirm
```

or use **⬆️ Check for updates / update** in the main menu.

- In a terminal (at least 24 rows × 60 columns) an update panel opens: after the signature and archive checks it shows the version, signature status, package (`components.lock`) differences and the **Dikkat** (caution) notes from the CHANGELOG at the top, and the release notes, confirmation and installation below. When a package version changes, a "what changed, which of my servers are affected" report is printed at the end of the install (also available with `./start.sh --components report [package [old-version]]`; read-only).
- The package is only the file from this repository's GitHub Releases, fetched over HTTPS and **signed with the
  maintainer's release key** (`ssh-keygen -Y verify`). If it cannot be verified nothing is unpacked or installed.
  If no signing key is configured, updating is **disabled**.
- Your servers (`servers/`), settings (`sshpocket.conf`) and keys are kept. A backup of the code (`.update-backup/`, no secrets) is taken first
  and restored on failure.
- It never downgrades, and rejects archives containing `..`, absolute paths or symlinks.
- A write-protected USB cannot be updated (by design).
- Note: the updater ships with `v2.2.0`; older copies need one manual update first.

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
