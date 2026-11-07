# Paketler

Paket: imzalı, kilitli (`components.lock`) bir bileşen (örn. `linux-hardening`, `utility`). Sözleşme: [package-standard.md](package-standard.md). Ana sayfa: [README](../README.md).

- **Paketler (altyapı):** imzalı, kilitli (`components.lock`) paketlerin güvenli kurulumu: `./sshpocket components list|sync|check-new|prune`; kurulu paketin eylemi: `./sshpocket components run <paket> <eylem> <sunucu>` (sunucuya bağlanır; önizleme/onay kapıları vardır); eski sürüm klasörlerini temizleme: `./sshpocket components prune [--dry-run] [--yes]` (etkin, bir önceki ve kilitteki sürüm kalır); güncellemeden sonra otomatik sorulur ve **Ayarlar › Eski paket sürümlerini temizle** menüsünden de yapılır; paket yazarları için sözleşme uyum denetimi: `./sshpocket components validate <paket|dizin|arşiv> [--strict]` (statik).

- **Sunucu türü ve izinli ağ:** paket platform listesi bildiriyorsa (linux-hardening 0.12.0: Ubuntu/Debian, Proxmox VE, Raspberry Pi), sunucu eklerken ve eski sunucularda paket menüsü ilk açılırken sunucu türü sorulur ("Şimdi değil" hiçbir şey yazmaz). **`hardening_ssh_allow_from` zorunludur:** boşken `preview`, `apply`, `pve_firewall_*` ssh'a gitmeden ⛔ ile durur (`audit` ve `rollback_*` etkilenmez); öneri sunucunun sizi gördüğü adres ve ağdan gelir, sessizce yazılmaz. Proxmox'ta `apply`, seçili platformun `hardening_pve_apply=true` değerini yalnızca o koşuya özel verir (uyarı sunucu adı onayından önce gösterilir, `.vars` dosyanıza yazılmaz). Ayrıntı: [docs/sunucu-ekleme-platform-secimi.md](sunucu-ekleme-platform-secimi.md), [docs/hardening-kullanim.md](hardening-kullanim.md).

Ne denendi, ne denenmedi: [test-edilen-ortamlar.md](test-edilen-ortamlar.md).
