#!/usr/bin/env bash
# release.sh - Etiketli sürümden imzalı release arşivi üretir (bakımcı aracı, yerelde çalışır).
#
#   1) VERSION'ı artırın, CHANGELOG'u güncelleyin, commit + etiket: git tag -a vX.Y.Z -m "..."
#   2) SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release ./scripts/release.sh
#   3) dist/ içindeki .tar.gz ve .tar.gz.sig dosyalarını GitHub Release'e yükleyin.
#
# Özel imza anahtarı ASLA repoya, CI'a veya sohbete konmaz. Ayrıntı: RELEASING.md
set -euo pipefail
cd "$(dirname "$0")/.."

NS="sshpocket-release"
ID="sshpocket-release"
KEY="${SSHPOCKET_SIGNING_KEY:-$HOME/.ssh/sshpocket_release}"
SIGNERS="sistem/release_signers"

die() { echo "HATA: $*" >&2; exit 1; }

VERSION=$(tr -d '[:space:]' < VERSION)
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION geçersiz: '$VERSION'"
TAG="v$VERSION"

[ -z "$(git status --porcelain)" ] || die "çalışma alanı temiz değil (commit edilmemiş değişiklik var)"
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null || die "etiket yok: $TAG (önce: git tag -a $TAG -m ...)"
[ "$(git rev-list -n1 "$TAG")" = "$(git rev-parse HEAD)" ] || die "$TAG şu anki commit'i göstermiyor"
grep -q "^## \[$VERSION\]" CHANGELOG.md || die "CHANGELOG'da [$VERSION] başlığı yok"
[ -f "$KEY" ] || die "imza anahtarı bulunamadı: $KEY (SSHPOCKET_SIGNING_KEY ile belirtin)"
grep -q '^[^#].*ssh-' "$SIGNERS" || die "$SIGNERS içinde public key yok; önce ekleyip commit edin"

mkdir -p dist
ARCHIVE="dist/sshpocket-$VERSION.tar.gz"
rm -f "$ARCHIVE" "$ARCHIVE.sig"
git archive --format=tar.gz --prefix="sshpocket-$VERSION/" -o "$ARCHIVE" "$TAG"
ssh-keygen -Y sign -f "$KEY" -n "$NS" "$ARCHIVE"

# Yayından önce kendi doğrulamamız: repodaki signers dosyasıyla geçmeli
ssh-keygen -Y verify -f "$SIGNERS" -I "$ID" -n "$NS" -s "$ARCHIVE.sig" < "$ARCHIVE" >/dev/null \
    || die "üretilen imza, $SIGNERS içindeki anahtarla DOĞRULANAMADI (yanlış anahtar?)"

echo ""
echo "✅ İmzalı paket hazır ve doğrulandı:"
ls -l "$ARCHIVE" "$ARCHIVE.sig"
echo "SHA-256: $(shasum -a 256 "$ARCHIVE" | cut -d' ' -f1)"
echo ""
echo "Sıradaki: GitHub'da $TAG release'ini açın ve şu iki dosyayı 'Assets'e yükleyin:"
echo "  $ARCHIVE"
echo "  $ARCHIVE.sig"
