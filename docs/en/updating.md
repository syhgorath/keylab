# Updating

Home: [README](../../README.en.md). The steps to verify your first download are in the README.

Never automatic; you start it:

```bash
./sshpocket update --check   # only looks (exit code 10 if a newer version exists)
./sshpocket update         # download → verify signature → show notes → install if you confirm
```

These commands are for the bare binary (`./sshpocket`). If you use `KeyLab.app`, see **in-app update** below (the old menu/fzf layout does not exist in the Go version).

- The command-line output shows the version, signature status, release notes and a confirmation prompt after the signature and archive checks. When a package version changes, a "what changed, which of my servers are affected" report is available with `./sshpocket components report [package [old-version]]` (read-only).
- The package is only the file from this repository's GitHub Releases, fetched over HTTPS and **signed with the
  maintainer's release key** (`ssh-keygen -Y verify`). If it cannot be verified nothing is unpacked or installed.
  If no signing key is configured, updating is **disabled**.
- Your servers (`servers/`), settings (`sshpocket.conf`) and keys are kept. A backup of the code (`.update-backup/`, no secrets) is taken first
  and restored on failure.
- It never downgrades, and rejects archives containing `..`, absolute paths or symlinks.
- A write-protected USB cannot be updated (by design).
- An update runs with the **installed version's code**: a fix to the updater only helps users who already have that version; older versions switch manually once (e.g. updating `KeyLab.app` from a USB stick works from 4.0.13 on).

## In-app update in KeyLab.app (4.0.11+)

**Settings > Update**: "Check for updates" shows the latest GitHub version and its release notes (KeyLab never goes online by itself); "Update and restart" downloads the package, verifies its signature with the release key, replaces `.app` as a whole (the old one is backed up under `.keylab-update-backup/`) and relaunches KeyLab. The button appears only when it runs as `KeyLab.app` and the app's folder is writable; use `keylab update` for the command-line binary. If macOS runs the app from a temporary path because of quarantine (App Translocation), move KeyLab.app to Applications (or the USB stick) and open it from there.

**Packages update separately:** package versions are independent of KeyLab updates; they come from the signed index (`sshpocket-index`) and are applied from the Packages page or with `components update | upgrade`. If a package's repository changes (the lock pins a package to a repo), the index and the embedded `components.lock` must point at the same repo; otherwise the verdict is `other-repo` and nothing is installed.

**Updating from USB:** on exFAT/FAT macOS writes `._*` sidecar files next to every item; from 4.0.13 on they are not treated as part of the package. A write-protected USB cannot be updated.
