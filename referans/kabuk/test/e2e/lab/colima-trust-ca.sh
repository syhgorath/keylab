#!/usr/bin/env bash
# colima-trust-ca.sh "<CA ortak adı>" [--yes] — macOS Keychain'deki bir CA sertifikasını YALNIZCA Colima VM'inin güven deposuna ekler.
# Ne zaman: bir antivirüs/güvenlik yazılımı HTTPS'i araya girip yeniden imzalıyorsa (örn. "ESET SSL Filter CA") macOS o sertifikaya güvenir
# ama Colima'nın Linux VM'i güvenmez; `docker pull` "certificate signed by unknown authority" der.
# Güvenlik: yalnızca AÇIK sertifika dışa aktarılır (gizli anahtar değil); ekleme öncesi ad, parmak izi ve "CA:TRUE" gösterilir ve onay istenir.
# Geri alma: `colima delete` (VM silinir) ya da VM'de /usr/local/share/ca-certificates/<ad>.crt silinip update-ca-certificates çalıştırılır.
set -eu
CN="${1:-}"; YES="${2:-}"
[ -n "$CN" ] || { echo "kullanım: $0 \"<CA ortak adı>\" [--yes]"; exit 2; }
command -v colima >/dev/null || { echo "colima bulunamadı"; exit 1; }
colima status >/dev/null 2>&1 || { echo "Colima çalışmıyor: colima start"; exit 1; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
for kc in /Library/Keychains/System.keychain "$HOME/Library/Keychains/login.keychain-db"; do
    security find-certificate -a -c "$CN" -p "$kc" 2>/dev/null || true
done > "$tmp/all.pem"
awk '/BEGIN CERT/{p=1} p{print} /END CERT/{exit}' "$tmp/all.pem" > "$tmp/ca.crt"
[ -s "$tmp/ca.crt" ] || { echo "Keychain'de '$CN' sertifikası bulunamadı"; exit 1; }
openssl x509 -in "$tmp/ca.crt" -noout -subject -dates -fingerprint -sha256
openssl x509 -in "$tmp/ca.crt" -noout -text | grep -q "CA:TRUE" || { echo "Bu bir CA sertifikası değil; eklenmedi."; exit 1; }
if [ "$YES" != "--yes" ]; then
    printf 'Bu CA, Colima VM içindeki TÜM HTTPS bağlantıları için güvenilir olacak. Eklensin mi? [e/H]: '
    read -r ans || exit 1
    case "$ans" in e|E|evet|Evet) ;; *) echo "İptal edildi."; exit 1 ;; esac
fi
safe="$(printf '%s' "$CN" | tr -c 'A-Za-z0-9' '-' | tr '[:upper:]' '[:lower:]')"
colima ssh -- sudo tee "/usr/local/share/ca-certificates/${safe}.crt" < "$tmp/ca.crt" >/dev/null
colima ssh -- sudo update-ca-certificates >/dev/null
colima ssh -- sudo systemctl restart docker
echo "✅ Eklendi: ${safe}.crt (yalnızca Colima VM'i). Denemek için: docker pull ubuntu:26.04"
