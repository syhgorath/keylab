#!/usr/bin/env bash
# build-release.sh - Tek ikili yayın varlıklarını üretir ve İMZALAR (bakımcı aracı, yerelde çalışır; hiçbir şey yüklemez/itmez).
#
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release go/scripts/build-release.sh [--dry-run]
#
# Çıktı (go/dist/): keylab-X.Y.Z-darwin-{arm64,amd64,universal} ve aynı adlarla .sig; geçiş için aynı dosyaların sshpocket-X.Y.Z-… adlı kopyaları da (eski istemciler) (SSHSIG, ad alanı sshpocket-release).
# Sürüm kökteki VERSION dosyasından alınır ve ikiliye gömülür (-X main.buildVersion). Özel anahtar ASLA repoya/CI'a konmaz.
# Sonraki adımlar sizde: etiket, GitHub Release'e yükleme (RELEASING.md).
set -euo pipefail
cd "$(dirname "$0")/.."            # go/
ROOT="$(cd .. && pwd)"
NS="sshpocket-release"; ID="sshpocket-release"
KEY="${SSHPOCKET_SIGNING_KEY:-$HOME/.ssh/sshpocket_release}"
SIGNERS="$ROOT/sistem/release_signers"; [ -f "$ROOT/release_signers" ] && SIGNERS="$ROOT/release_signers"
die() { echo "HATA: $*" >&2; exit 1; }

DRY=0; [ "${1:-}" = "--dry-run" ] && DRY=1
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION geçersiz: '$VERSION'"
command -v go >/dev/null || die "go bulunamadı"
grep -q '^[^#].*ssh-' "$SIGNERS" || die "$SIGNERS içinde public key yok"
if [ "$DRY" = 1 ]; then
    echo "(deneme) $VERSION için derlenecek: darwin/arm64, darwin/amd64 → universal; imza anahtarı: $KEY"
    [ -f "$KEY" ] && echo "(deneme) anahtar var" || echo "(deneme) UYARI: anahtar bulunamadı"
    exit 0
fi
[ -f "$KEY" ] || die "imza anahtarı bulunamadı: $KEY (SSHPOCKET_SIGNING_KEY ile belirtin)"

rm -rf dist && mkdir -p dist
build() { # goos goarch out
    CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "-s -w -X main.buildVersion=$VERSION" -o "$3" .
}
A="dist/keylab-$VERSION-darwin-arm64"; B="dist/keylab-$VERSION-darwin-amd64"; U="dist/keylab-$VERSION-darwin-universal"
build darwin arm64 "$A"; build darwin amd64 "$B"
assets=("$A" "$B")
if command -v lipo >/dev/null; then
    lipo -create -output "$U" "$A" "$B"
    command -v codesign >/dev/null && codesign --force -s - "$U"
    assets+=("$U")
else
    echo "UYARI: lipo yok; universal ikili üretilmedi (USB için tek dosya gerekir)"
fi
for f in "${assets[@]}"; do
    chmod 755 "$f"
    ssh-keygen -Y sign -f "$KEY" -n "$NS" "$f"
    ssh-keygen -Y verify -f "$SIGNERS" -I "$ID" -n "$NS" -s "$f.sig" < "$f" >/dev/null \
        || die "$f imzası $SIGNERS ile DOĞRULANAMADI (yanlış anahtar?)"
done
# geçiş dönemi: eski adlı kopyalar (içerik ve imza aynı; imza içeriğe bağlıdır, ada değil). Eski istemciler yalnız sshpocket-… arar.
for f in "${assets[@]}"; do
    old="dist/sshpocket-${f#dist/keylab-}"
    cp "$f" "$old"; cp "$f.sig" "$old.sig"
    ssh-keygen -Y verify -f "$SIGNERS" -I "$ID" -n "$NS" -s "$old.sig" < "$old" >/dev/null || die "$old imzası doğrulanamadı"
    assets+=("$old")
done
echo ""
echo "✅ İmzalı varlıklar hazır ve doğrulandı:"
ls -l "${assets[@]/%/}" ; for f in "${assets[@]}"; do ls -l "$f.sig"; done
for f in "${assets[@]}"; do echo "SHA-256 $(shasum -a 256 "$f" | cut -d' ' -f1)  $(basename "$f")"; done
echo ""
echo "Not: bu makinede çalışan ikili sürümünü doğrulamak için: ${assets[0]} version"
echo "Sıradaki: GitHub'da v$VERSION Release'ini açın; yukarıdaki dosyaları ve .sig'lerini Assets'e yükleyin."
