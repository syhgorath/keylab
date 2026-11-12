#!/usr/bin/env bash
# start.command - Finder'da çift tıklayınca sshpocket'i başlatır (macOS). USB kökünde `sshpocket` ikilisinin yanında durur.
# macOS USB takılınca otomatik çalıştırma yapmaz; bu yalnızca tek tıkla başlatma kolaylığıdır.
# İlk açılışta Gatekeeper onay isteyebilir (sağ tık > Aç).
cd "$(dirname "$0")" || exit 1
[ -x ./sshpocket ] || { echo "HATA: ./sshpocket bulunamadı (Release'ten keylab-X.Y.Z-darwin-universal dosyasını 'sshpocket' adıyla bu klasöre koyun)" >&2; exit 1; }
exec ./sshpocket "$@"
