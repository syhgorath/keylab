#!/usr/bin/env bash
# package-app.sh - universal ikiliden KeyLab.app üretir (ad-hoc imzalı) ve zip'ler (bakımcı aracı, yalnız macOS; build-release.sh çağırır).
#
#   bash go/scripts/package-app.sh <universal-ikili> <sürüm> <çıktı-klasörü>
#
# Çıktı: <çıktı>/KeyLab.app  ve  <çıktı>/keylab-<sürüm>-macos.zip  (zip İMZASIZ üretilir; .sig'i build-release.sh atar).
# Uygulama paketi yalnızca ikiliyi, Info.plist'i ve simgeyi içerir: başlatıcı betik YOK, çift tıklayınca ikili doğrudan çalışır, veri klasörünü
# kendisi bulur (yanında servers/ varsa taşınabilir, yoksa ~/.keylab; bkz. go/datadirs.go). Dock'ta simge çıkar; pencere/menü çubuğu yoktur
# (arayüz tarayıcıda açılır), çıkmak için arayüzdeki Çıkış düğmesi ya da Dock > Çık.
# İmza ad-hoc'tur (Apple Developer ID / noter onayı YOK): internetten indirilen kopya ilk açılışta Gatekeeper uyarısı verir
# (README "Kurulum": sağ tık > Aç ya da Sistem Ayarları > Gizlilik ve Güvenlik > Yine de aç).
set -euo pipefail

die() { echo "HATA: $*" >&2; exit 1; }
[ $# -eq 3 ] || die "kullanım: package-app.sh <universal-ikili> <sürüm> <çıktı-klasörü>"
BIN="$1"; VER="$2"; OUT="$3"
[ "$(uname -s)" = "Darwin" ] || die "yalnız macOS'ta çalışır (sips, iconutil, codesign, ditto gerekir)"
[ -f "$BIN" ] || die "ikili yok: $BIN"
[[ "$VER" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "sürüm geçersiz: '$VER'"
for c in sips iconutil codesign ditto plutil; do command -v "$c" >/dev/null || die "$c yok"; done

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOGO="$ROOT/img/logo.jpg"
[ -f "$LOGO" ] || die "simge kaynağı yok: $LOGO"

mkdir -p "$OUT"
APP="$OUT/KeyLab.app"
ZIP="$OUT/keylab-$VER-macos.zip"
rm -rf "$APP" "$ZIP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/KeyLab"
chmod 755 "$APP/Contents/MacOS/KeyLab"

# simge: logo.jpg (512x512) → AppIcon.icns
ICONSET="$(mktemp -d)/AppIcon.iconset"
mkdir -p "$ICONSET"
for s in 16 32 64 128 256 512; do
    sips -s format png -z "$s" "$s" "$LOGO" --out "$ICONSET/tmp_$s.png" >/dev/null
done
cp "$ICONSET/tmp_16.png"  "$ICONSET/icon_16x16.png"
cp "$ICONSET/tmp_32.png"  "$ICONSET/icon_16x16@2x.png"
cp "$ICONSET/tmp_32.png"  "$ICONSET/icon_32x32.png"
cp "$ICONSET/tmp_64.png"  "$ICONSET/icon_32x32@2x.png"
cp "$ICONSET/tmp_128.png" "$ICONSET/icon_128x128.png"
cp "$ICONSET/tmp_256.png" "$ICONSET/icon_128x128@2x.png"
cp "$ICONSET/tmp_256.png" "$ICONSET/icon_256x256.png"
cp "$ICONSET/tmp_512.png" "$ICONSET/icon_256x256@2x.png"
cp "$ICONSET/tmp_512.png" "$ICONSET/icon_512x512.png"
rm -f "$ICONSET"/tmp_*.png
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns" || die "simge üretilemedi"
rm -rf "$(dirname "$ICONSET")"

cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key><string>tr</string>
    <key>CFBundleExecutable</key><string>KeyLab</string>
    <key>CFBundleIconFile</key><string>AppIcon</string>
    <key>CFBundleIdentifier</key><string>io.github.syhgorath.keylab</string>
    <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
    <key>CFBundleName</key><string>KeyLab</string>
    <key>CFBundleDisplayName</key><string>KeyLab</string>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>CFBundleShortVersionString</key><string>$VER</string>
    <key>CFBundleVersion</key><string>$VER</string>
    <key>LSApplicationCategoryType</key><string>public.app-category.developer-tools</string>
    <key>NSHighResolutionCapable</key><true/>
    <key>NSHumanReadableCopyright</key><string>MIT License</string>
</dict>
</plist>
EOF
plutil -lint "$APP/Contents/Info.plist" >/dev/null || die "Info.plist geçersiz"

codesign --force --deep -s - "$APP" || die "ad-hoc imza atılamadı"
codesign --verify --deep --strict "$APP" || die "imza doğrulanamadı"

# zip: ditto, çalıştırma bitlerini ve paket yapısını korur (zip komutu bazı meta verileri kaybeder)
ditto -c -k --keepParent "$APP" "$ZIP"
[ -s "$ZIP" ] || die "zip üretilmedi"
echo "✅ $APP ve $ZIP hazır"
