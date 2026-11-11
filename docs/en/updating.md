# Updating

Home: [README](../../README.en.md). The steps to verify your first download are in the README.

Never automatic; you start it:

```bash
./sshpocket update --check   # only looks (exit code 10 if a newer version exists)
./sshpocket update         # download → verify signature → show notes → install if you confirm
```

or use **⬆️ Check for updates / update** in the main menu (in the fzf layout: the sshpocket row under **📦 Packages and updates**).

- In a terminal (at least 24 rows × 60 columns) an update panel opens: after the signature and archive checks it shows the version, signature status, package (`components.lock`) differences and the **Dikkat** (caution) notes from the CHANGELOG at the top, and the release notes, confirmation and installation below. When a package version changes, a "what changed, which of my servers are affected" report is printed at the end of the install (also available with `./sshpocket components report [package [old-version]]`; read-only).
- The package is only the file from this repository's GitHub Releases, fetched over HTTPS and **signed with the
  maintainer's release key** (`ssh-keygen -Y verify`). If it cannot be verified nothing is unpacked or installed.
  If no signing key is configured, updating is **disabled**.
- Your servers (`servers/`), settings (`sshpocket.conf`) and keys are kept. A backup of the code (`.update-backup/`, no secrets) is taken first
  and restored on failure.
- It never downgrades, and rejects archives containing `..`, absolute paths or symlinks.
- A write-protected USB cannot be updated (by design).
- Note: the updater ships with `v2.2.0`; older copies need one manual update first.
