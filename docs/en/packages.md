# Packages

A package is a signed, locked (`components.lock`) component (e.g. `linux-hardening`, `utility`). Contract: [package-standard.md](../package-standard.md) (Turkish). Home: [README](../../README.en.md).

- **Packages (infrastructure):** safe install of signed, locked (`components.lock`) packages: `./sshpocket components list|sync|check-new|prune`; clean up old version folders with `./sshpocket components prune [--dry-run] [--yes]` (the active, the previous and the locked version are kept); it is also offered after an update and available from **Settings › Eski paket sürümlerini temizle** (clean up old package versions); run an installed package's action: `./sshpocket components run <package> <action> <server>` (connects to the server; preview/confirmation gates apply).

- **Server type and allowed network:** if a package declares a platform list (linux-hardening 0.12.0: Ubuntu/Debian, Proxmox VE, Raspberry Pi), the server type is asked when adding a server and, for older servers, the first time the package menu opens ("Not now" writes nothing). **`hardening_ssh_allow_from` is required:** while empty, `preview`, `apply` and `pve_firewall_*` stop with ⛔ before any ssh (`audit` and `rollback_*` are not affected); suggestions come from the address the server sees you at and are never written silently. On Proxmox, `apply` passes the selected platform's `hardening_pve_apply=true` for that run only (the warning is shown before the server-name confirmation and nothing is written to your `.vars` file). Details: [docs/sunucu-ekleme-platform-secimi.md](../sunucu-ekleme-platform-secimi.md) and [docs/hardening-kullanim.md](../hardening-kullanim.md) (Turkish).

What was and was not tried: [tested-environments.md](tested-environments.md).
