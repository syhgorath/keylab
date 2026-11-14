#!/usr/bin/env bash
# release-publish.sh - sürüm yayınını TEK komuta indirir (bakımcı aracı, yerelde çalışır): kontroller → derle + İMZALA (build-release.sh)
# → etiket → GitHub Release (varlıklar + CHANGELOG notu) → yüklenen dosyaların özet doğrulaması.
#
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release go/scripts/release-publish.sh --dry-run   # ne yapacağını gösterir, HİÇBİR ŞEY yapmaz
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release go/scripts/release-publish.sh              # onay ister, sonra derler, imzalar, etiketler, yayınlar
#   --yes (onay sorma), --no-ci-check (main CI kontrolünü atla: önerilmez)
#
# Önkoşul: sürüm PR'ı birleşmiş, yerel main güncel, main'in CI'ı yeşil. Özel imza anahtarı ASLA repoya, CI'a veya sohbete konmaz; yalnızca
# build-release.sh'e SSHPOCKET_SIGNING_KEY ile aktarılır (passphrase sorulur). Sıra bilerek "önce derle+imzala, sonra etiketle":
# imza başarısız olursa uzakta Release'siz etiket kalmaz. Ayrıntı: RELEASING.md
set -euo pipefail
cd "$(dirname "$0")/../.."   # depo kökü

die() { echo "HATA: $*" >&2; exit 1; }
DRY=0; YES=0; NOCI=0
for a in "$@"; do
    case "$a" in
        --dry-run) DRY=1 ;;
        --yes) YES=1 ;;
        --no-ci-check) NOCI=1 ;;
        -h|--help) sed -n 2,9p "$0"; exit 0 ;;
        *) die "bilinmeyen bağımsız değişken: $a (kullanım: --dry-run, --yes, --no-ci-check)" ;;
    esac
done
command -v git >/dev/null || die "git yok"
command -v gh >/dev/null || die "gh (GitHub CLI) yok"
command -v ssh-keygen >/dev/null || die "ssh-keygen yok"
command -v go >/dev/null || die "go yok"

VERSION=$(tr -d '[:space:]' < VERSION)
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION geçersiz: '$VERSION'"
TAG="v$VERSION"
KEY="${SSHPOCKET_SIGNING_KEY:-$HOME/.ssh/sshpocket_release}"
SUMOF() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "main dalında değilsiniz"
[ -z "$(git status --porcelain --untracked-files=no)" ] || die "çalışma alanı temiz değil (commit edilmemiş değişiklik var)"
git fetch -q origin main --tags || die "git fetch başarısız"
HEAD_SHA=$(git rev-parse HEAD)
[ "$HEAD_SHA" = "$(git rev-parse origin/main)" ] || die "HEAD origin/main ile aynı değil (önce: git pull --ff-only)"
grep -q "^## \[$VERSION\]" CHANGELOG.md || die "CHANGELOG.md içinde [$VERSION] başlığı yok"
# imza anahtarı ve güven noktası ETİKET İTİLMEDEN önce denetlenir
[ -f "$KEY" ] || die "imza anahtarı bulunamadı: $KEY (SSHPOCKET_SIGNING_KEY ile belirtin)"
grep -q '^[^#].*ssh-' release_signers || die "release_signers içinde public key yok"

if gh release view "$TAG" >/dev/null 2>&1; then die "GitHub Release $TAG zaten var"; fi
if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
    [ "$(git rev-list -n1 "$TAG")" = "$HEAD_SHA" ] || die "yerel $TAG etiketi şu anki commit'i göstermiyor"
fi
if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
    [ "$(git ls-remote origin "refs/tags/$TAG^{}" | cut -f1)" = "$HEAD_SHA" ] || die "uzaktaki $TAG etiketi şu anki commit'i göstermiyor"
fi

if [ "$NOCI" = 0 ]; then
    CI=$(gh run list --branch main --workflow ci.yml --limit 1 --json status,conclusion,headSha --jq '.[0] | "\(.status) \(.conclusion) \(.headSha)"') || die "gh run list başarısız"
    [ "$CI" = "completed success $HEAD_SHA" ] || die "main CI yeşil değil ya da şu anki commit'e ait değil: '$CI' (beklenen: completed success $HEAD_SHA); birleşmeden sonra CI'ın bitmesini bekleyin"
fi

# Sürüm notları: CHANGELOG.md'deki [VERSION] bölümü
NOTES=$(mktemp)
trap 'rm -f "$NOTES"' EXIT
awk -v v="$VERSION" '$0 ~ "^## \\[" v "\\]" {f=1; next} /^## \[/ {f=0} f' CHANGELOG.md | sed '/./,$!d' | awk '{l[NR]=$0} END{n=NR; while (n>0 && l[n]=="") n--; for (i=1;i<=n;i++) print l[i]}' > "$NOTES"
[ -s "$NOTES" ] || die "CHANGELOG.md [$VERSION] bölümü boş"

echo "Sürüm: $TAG (commit ${HEAD_SHA:0:7})"
echo "Yapılacaklar:"
echo "  1) go/scripts/build-release.sh  (derle: darwin arm64+amd64+universal, İMZALA, kendi doğrulaması; passphrase sorulur)"
echo "  2) git tag -a $TAG -m \"KeyLab $VERSION\" && git push origin $TAG"
echo "  3) gh release create $TAG <go/dist varlıkları ve .sig'leri> --title $TAG --notes-file <CHANGELOG [$VERSION] bölümü, $(wc -l < "$NOTES" | tr -d ' ') satır>"
echo "  4) yüklenen dosyaların SHA-256 özetini yerel dosyalarla karşılaştır"
echo "  (USB'yi bu betikten sonra kendiniz güncelleyin)"
if [ "$DRY" = 1 ]; then echo "[dry-run] hiçbir şey yapılmadı."; exit 0; fi
if [ "$YES" = 0 ]; then
    read -r -p "Devam edilsin mi? (etiket itilir, Release yayımlanır) [y/N] " ans </dev/tty || ans=""
    [ "$ans" = "y" ] || [ "$ans" = "Y" ] || die "iptal edildi"
fi

# 1) derle + imzala (başarısız olursa hiçbir şey itilmemiş olur)
SSHPOCKET_SIGNING_KEY="$KEY" bash go/scripts/build-release.sh   # (depoda çalıştırma biti yok; bash ile)
U="go/dist/keylab-$VERSION-darwin-universal"
[ -f "$U" ] && [ -f "$U.sig" ] || die "universal ikili ya da imzası üretilmedi (lipo yok mu?)"
"$U" version 2>&1 | grep -qF "$VERSION" || die "$U version çıktısında $VERSION yok"

assets=()
for f in go/dist/*-"$VERSION"-darwin-*; do
    case "$f" in *.sig) ;; *) assets+=("$f" "$f.sig") ;; esac
done
[ "${#assets[@]}" -gt 0 ] || die "go/dist içinde yayın varlığı yok"
for f in "${assets[@]}"; do [ -f "$f" ] || die "eksik varlık: $f"; done

# 2) etiket
if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
    git tag -a "$TAG" -m "KeyLab $VERSION"
fi
git push origin "$TAG"

# 3) Release
gh release create "$TAG" "${assets[@]}" --title "$TAG" --notes-file "$NOTES" --verify-tag

# 4) yüklenenlerin özeti yerelle AYNI olmalı
REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
REMOTE=$(gh api "repos/$REPO/releases/tags/$TAG" --jq '.assets[] | "\(.name) \(.digest)"') || die "Release varlıkları okunamadı"
for f in "${assets[@]}"; do
    want="sha256:$(SUMOF "$f")"
    got=$(printf '%s\n' "$REMOTE" | awk -v n="$(basename "$f")" '$1 == n {print $2}')
    [ -n "$got" ] || die "Release'te $(basename "$f") yok"
    [ "$got" = "$want" ] || die "$(basename "$f") özeti uyuşmuyor: Release=$got yerel=$want"
done
echo ""
echo "✅ $TAG yayımlandı ve yüklenen ${#assets[@]} dosyanın özeti yerelle aynı."
echo "Evrensel ikili SHA-256: $(SUMOF "$U")"
echo "Sıradaki: eski sürümlü bir kopyada './sshpocket update --check' ile deneyin; USB'yi güncelleyin."
