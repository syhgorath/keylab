#!/usr/bin/env bash
# build-release.sh - Tek ikili yayın varlıklarını üretir ve İMZALAR (bakımcı aracı, yerelde çalışır; hiçbir şey yüklemez/itmez).
#
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release go/scripts/build-release.sh [--dry-run]
#
# Çıktı (go/dist/): keylab-X.Y.Z-darwin-arm64 ve .sig (SSHSIG, ad alanı sshpocket-release) + KeyLab.app zip'i. Yalnız Apple Silicon ve güncel macOS (27+):
# amd64/universal ve eski sshpocket-… adlı kopyalar 4.0.5'ten itibaren üretilmez.
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
bash scripts/sync-embedded.sh --check || die "gömülü güven dosyaları kökle aynı değil (ikiliye yanlış anahtar/kilit gömülmesin)"
grep -q '^[^#].*ssh-' "$SIGNERS" || die "$SIGNERS içinde public key yok"
if [ "$DRY" = 1 ]; then
    echo "(deneme) $VERSION için derlenecek: darwin/arm64 (yalnız Apple Silicon); imza anahtarı: $KEY"
    [ -f "$KEY" ] && echo "(deneme) anahtar var" || echo "(deneme) UYARI: anahtar bulunamadı"
    exit 0
fi
[ -f "$KEY" ] || die "imza anahtarı bulunamadı: $KEY (SSHPOCKET_SIGNING_KEY ile belirtin)"

rm -rf dist && mkdir -p dist
# macOS ikilisi GÖMÜLÜ PENCERE (WKWebView; CGO + -tags webview) ile derlenir: uygulama kendi penceresinde açılır, kapatınca süreç biter, Chrome gerekmez.
# clang -arch arm64 (Xcode Command Line Tools gerekir). Pencere derlenemezse sürüm DURUR:
# pencere olmadan yayınlamak eski "Chrome penceresi" davranışını sessizce geri getirirdi.
build() { # goos goarch out
    if [ "$1" = darwin ]; then
        local cc="clang -arch arm64"
        CGO_ENABLED=1 CC="$cc" GOOS="$1" GOARCH="$2" go build -trimpath -tags webview -ldflags "-s -w -X main.buildVersion=$VERSION" -o "$3" . \
            || die "pencere (webview) derlemesi başarısız: $1/$2 (Xcode Command Line Tools: xcode-select --install)"
    else
        CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "-s -w -X main.buildVersion=$VERSION" -o "$3" .
    fi
}
A="dist/keylab-$VERSION-darwin-arm64"
build darwin arm64 "$A"
command -v codesign >/dev/null && codesign --force -s - "$A"
assets=("$A")
for f in "${assets[@]}"; do
    chmod 755 "$f"
    ssh-keygen -Y sign -f "$KEY" -n "$NS" "$f"
    ssh-keygen -Y verify -f "$SIGNERS" -I "$ID" -n "$NS" -s "$f.sig" < "$f" >/dev/null \
        || die "$f imzası $SIGNERS ile DOĞRULANAMADI (yanlış anahtar?)"
done
# macOS uygulama paketi: KeyLab.app (arm64 ikiliden, ad-hoc imzalı) → keylab-X.Y.Z-macos.zip; zip de aynı anahtarla imzalanır.
if [ -f "$A" ] && [ "$(uname -s)" = "Darwin" ]; then
    bash scripts/package-app.sh "$A" "$VERSION" dist
    Z="dist/keylab-$VERSION-macos.zip"
    ssh-keygen -Y sign -f "$KEY" -n "$NS" "$Z"
    ssh-keygen -Y verify -f "$SIGNERS" -I "$ID" -n "$NS" -s "$Z.sig" < "$Z" >/dev/null || die "$Z imzası doğrulanamadı"
    assets+=("$Z")
else
    echo "UYARI: KeyLab.app/zip üretilmedi (arm64 ikili ya da macOS yok)"
fi
echo ""
echo "✅ İmzalı varlıklar hazır ve doğrulandı:"
ls -l "${assets[@]/%/}" ; for f in "${assets[@]}"; do ls -l "$f.sig"; done
for f in "${assets[@]}"; do echo "SHA-256 $(shasum -a 256 "$f" | cut -d' ' -f1)  $(basename "$f")"; done
echo ""
echo "Not: bu makinede çalışan ikili sürümünü doğrulamak için: ${assets[0]} version"
echo "Sıradaki: GitHub'da v$VERSION Release'ini açın; yukarıdaki dosyaları ve .sig'lerini Assets'e yükleyin."
