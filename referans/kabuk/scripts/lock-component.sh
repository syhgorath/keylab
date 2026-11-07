#!/usr/bin/env bash
# lock-component.sh - BAKIMCI aracı: bir paket sürümünü components.lock'a güvenle yazar.
#
#   bash scripts/lock-component.sh <id> <sürüm> [--repo sahip/depo] [--signers "<satır>"] [--write]
#
# Adımlar: paketi + .sig'i GitHub Release'ten (yalnızca HTTPS) indirir → arşiv denetimi → imzayı
# (ad alanı sshpocket-component) doğrular → arşivi açıp component.json (kimlik+sürüm+şema) doğrular →
# sha256'yı hesaplar → components.lock girdisini üretir. --write yoksa YALNIZCA gösterir (dosyaya dokunmaz).
# Kurulamayacak bir paket (component.json yok/geçersiz) kilide ASLA yazılmaz.
#
# Güvenilen imzacı: mevcut girdinin signers satırı; yoksa sistem/release_signers'taki (tek imza anahtarı) açık anahtar;
# ya da --signers ile açıkça verilir. Test için: LOCK_COMPONENT_ARCHIVE / LOCK_COMPONENT_SIG indirmeyi atlar
# (doğrulamalar yine yapılır); LOCK_RELEASE_SIGNERS, release_signers yolunu değiştirir.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export SISTEM_PATH="$ROOT/sistem"
# shellcheck source=../sistem/Helpers/Components.sh
source "$ROOT/sistem/Helpers/Components.sh"
# shellcheck source=../sistem/Helpers/ComponentsInstall.sh
source "$ROOT/sistem/Helpers/ComponentsInstall.sh"
# shellcheck source=../sistem/Helpers/ComponentsManifest.sh
source "$ROOT/sistem/Helpers/ComponentsManifest.sh"

die() { echo "HATA: $*" >&2; exit 1; }

[ $# -ge 2 ] || die "kullanım: lock-component.sh <id> <sürüm> [--repo sahip/depo] [--signers \"<satır>\"] [--write]"
id="$1"; ver="$2"; shift 2
repo=""; signers=""; write=0
while [ $# -gt 0 ]; do
    case "$1" in
        --repo) repo="${2:-}"; shift 2 ;;
        --signers) signers="${2:-}"; shift 2 ;;
        --write) write=1; shift ;;
        *) die "bilinmeyen seçenek: $1" ;;
    esac
done
comp_id_valid "$id" || die "geçersiz paket kimliği"
comp_version_valid "$ver" || die "geçersiz sürüm (X.Y.Z)"

lock="$(comp_lock_file)"
# referans/kabuk/components.lock gerçek kilide sembolik bağdır: yazarken bağı EZMEMEK için hedef yola çözülür
if [ -L "$lock" ]; then
    _t="$(readlink "$lock")"
    case "$_t" in /*) lock="$_t" ;; *) lock="$(cd "$(dirname "$lock")" && cd "$(dirname "$_t")" && pwd -P)/$(basename "$_t")" ;; esac
fi
if [ -f "$lock" ]; then comp_lock_validate || die "mevcut components.lock geçersiz"; fi
if [ -z "$repo" ]; then repo=$(comp_get "$id" repo); fi
[ -n "$repo" ] || die "depo bilinmiyor: --repo sahip/depo verin"
comp_repo_valid "$repo" || die "geçersiz depo adı"

if [ -z "$signers" ]; then signers=$(comp_get "$id" signers); fi
if [ -z "$signers" ]; then
    rs="${LOCK_RELEASE_SIGNERS:-$ROOT/sistem/release_signers}"
    key=$(grep -E '^[^#].*ssh-ed25519 ' "$rs" 2>/dev/null | head -n1 | sed -E 's/.*(ssh-ed25519 [A-Za-z0-9+\/]+=*).*/\1/')
    [ -n "$key" ] || die "imzacı anahtarı bulunamadı ($rs); --signers verin"
    signers="sshpocket-component namespaces=\"sshpocket-component\" $key"
fi
comp_signers_valid "$signers" || die "signers satırı geçersiz"

work=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-lock.XXXXXX")
trap 'rm -r -- "${work:?}"' EXIT
archive="$work/pkg.tar.gz"; sig="$work/pkg.tar.gz.sig"
if [ -n "${LOCK_COMPONENT_ARCHIVE:-}" ]; then
    cp "$LOCK_COMPONENT_ARCHIVE" "$archive"; cp "${LOCK_COMPONENT_SIG:?LOCK_COMPONENT_SIG gerekli}" "$sig"
else
    url="https://github.com/${repo}/releases/download/v${ver}/${id}-${ver}.tar.gz"
    comp_download "$url" "$archive" || die "indirilemedi: $url"
    comp_download "$url.sig" "$sig" || die "imza indirilemedi: $url.sig"
fi

comp_check_archive "$archive" "$id-$ver" || die "arşiv denetimi başarısız"
printf '%s\n' "$signers" > "$work/signers"
ssh-keygen -Y verify -f "$work/signers" -I "$COMP_NAMESPACE" -n "$COMP_NAMESPACE" -s "$sig" < "$archive" >/dev/null 2>&1 \
    || die "imza doğrulanamadı (signers/ad alanı/paket uyuşmuyor)"

mkdir "$work/x"
tar -xzf "$archive" -C "$work/x" || die "arşiv açılamadı"
[ -f "$work/x/$id-$ver/component.json" ] \
    || die "paket component.json içermiyor; kurulamaz, kilide yazılmadı (bkz. linux-hardening#30)"
comp_manifest_info "$work/x/$id-$ver/component.json" "$id" "$ver" >/dev/null \
    || die "component.json geçersiz (yukarıdaki nedenler); kilide yazılmadı"

sha=$(comp_sha256_of "$archive")
entry=$(printf '[%s]\nrepo = %s\nversion = %s\nsha256 = %s\nsigners = %s\n' "$id" "$repo" "$ver" "$sha" "$signers")
echo "$entry"
echo ""
if [ "$write" -ne 1 ]; then echo "(yalnızca gösterildi; yazmak için --write)"; exit 0; fi

# Girdiyi değiştir/ekle (diğerleri aynen kalır); doğrula; atomik yaz
new="$work/lock.new"
{
    if [ -f "$lock" ]; then
        awk -v id="$id" '
            /^\[.*\]$/ { skip = ($0 == "[" id "]") }
            !skip { print }' "$lock"
    fi
    printf '\n%s\n' "$entry"
} | cat -s > "$new"
SSHPOCKET_COMPONENTS_LOCK="$new" comp_lock_validate || die "üretilen kilit dosyası geçersiz; yazılmadı"
cp "$new" "$lock.tmp" && mv -f "$lock.tmp" "$lock"
echo "✅ components.lock güncellendi: $id $ver"
