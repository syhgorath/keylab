#!/usr/bin/env bash
# release-publish.sh - release.sh'in ÖNCESİNİ ve SONRASINI yapar (bakımcı aracı, yerelde çalışır): kontroller, etiket, imzalı paket (release.sh),
# GitHub Release açma ve yüklenen dosyaların özet doğrulaması.
#
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release scripts/release-publish.sh --dry-run   # ne yapacağını gösterir, HİÇBİR ŞEY yapmaz
#   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release scripts/release-publish.sh              # onay ister, sonra etiketler, imzalar, Release açar
#   --yes (onay sorma), --no-ci-check (main CI kontrolünü atla: önerilmez)
#
# Önkoşul: sürüm PR'ı birleşmiş, yerel main güncel, main'in CI'ı yeşil. Özel imza anahtarı ASLA repoya, CI'a veya sohbete konmaz;
# yalnızca release.sh'e SSHPOCKET_SIGNING_KEY ile aktarılır (passphrase sorulur). USB güncellemesi bu betiğin işi değildir (menüden yapılır).
# Ayrıntı: RELEASING.md
set -euo pipefail
cd "$(dirname "$0")/.."

die() { echo "HATA: $*" >&2; exit 1; }
DRY=0; YES=0; NOCI=0
for a in "$@"; do
    case "$a" in
        --dry-run) DRY=1 ;;
        --yes) YES=1 ;;
        --no-ci-check) NOCI=1 ;;
        -h|--help) sed -n 2,10p "$0"; exit 0 ;;
        *) die "bilinmeyen bağımsız değişken: $a (kullanım: --dry-run, --yes, --no-ci-check)" ;;
    esac
done
command -v git >/dev/null || die "git yok"
command -v gh >/dev/null || die "gh (GitHub CLI) yok"
command -v ssh-keygen >/dev/null || die "ssh-keygen yok"

VERSION=$(tr -d '[:space:]' < VERSION)
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION geçersiz: '$VERSION'"
TAG="v$VERSION"
ARCHIVE="dist/sshpocket-$VERSION.tar.gz"
KEY="${SSHPOCKET_SIGNING_KEY:-$HOME/.ssh/sshpocket_release}"
SUMOF() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "main dalında değilsiniz"
[ -z "$(git status --porcelain)" ] || die "çalışma alanı temiz değil (commit edilmemiş değişiklik var)"
git fetch -q origin main --tags || die "git fetch başarısız"
HEAD_SHA=$(git rev-parse HEAD)
[ "$HEAD_SHA" = "$(git rev-parse origin/main)" ] || die "HEAD origin/main ile aynı değil (önce: git pull --ff-only)"
grep -q "^## \[$VERSION\]" CHANGELOG.md || die "CHANGELOG.md içinde [$VERSION] başlığı yok"
# release.sh'in anahtar denetimini ETİKET İTİLMEDEN önce de yap: etiket itildikten sonra imza başarısız olursa etiket Release'siz kalırdı
[ -f "$KEY" ] || die "imza anahtarı bulunamadı: $KEY (SSHPOCKET_SIGNING_KEY ile belirtin)"
grep -q '^[^#].*ssh-' sistem/release_signers || die "sistem/release_signers içinde public key yok"

if gh release view "$TAG" >/dev/null 2>&1; then die "GitHub Release $TAG zaten var"; fi
if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
    [ "$(git rev-list -n1 "$TAG")" = "$HEAD_SHA" ] || die "yerel $TAG etiketi şu anki commit'i göstermiyor"
fi
if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
    [ "$(git ls-remote origin "refs/tags/$TAG^{}" | cut -f1)" = "$HEAD_SHA" ] || die "uzaktaki $TAG etiketi şu anki commit'i göstermiyor"
fi

if [ "$NOCI" = 0 ]; then
    CI=$(gh run list --branch main --limit 1 --json status,conclusion,headSha --jq '.[0] | "\(.status) \(.conclusion) \(.headSha)"') || die "gh run list başarısız"
    [ "$CI" = "completed success $HEAD_SHA" ] || die "main CI yeşil değil ya da şu anki commit'e ait değil: '$CI' (beklenen: completed success $HEAD_SHA); birleşmeden sonra CI'ın bitmesini bekleyin"
fi

# Sürüm notları: CHANGELOG.md'deki [VERSION] bölümü
NOTES=$(mktemp)
trap 'rm -f "$NOTES"' EXIT
awk -v v="$VERSION" '$0 ~ "^## \\[" v "\\]" {f=1; next} /^## \[/ {f=0} f' CHANGELOG.md | sed '/./,$!d' | awk '{l[NR]=$0} END{n=NR; while (n>0 && l[n]=="") n--; for (i=1;i<=n;i++) print l[i]}' > "$NOTES"
[ -s "$NOTES" ] || die "CHANGELOG.md [$VERSION] bölümü boş"

echo "Sürüm: $TAG (commit ${HEAD_SHA:0:7})"
echo "Yapılacaklar:"
echo "  1) git tag -a $TAG -m \"sshpocket $VERSION\" && git push origin $TAG"
echo "  2) scripts/release.sh  (paket + imza + kendi doğrulaması; passphrase sorulur)"
echo "  3) gh release create $TAG $ARCHIVE $ARCHIVE.sig --title $TAG --notes-file <CHANGELOG [$VERSION] bölümü, $(wc -l < "$NOTES" | tr -d ' ') satır>"
echo "  4) yüklenen dosyaların SHA-256 özetini yerel dosyalarla karşılaştır"
echo "  (USB'yi bu betikten sonra menüden güncelleyin)"
if [ "$DRY" = 1 ]; then echo "[dry-run] hiçbir şey yapılmadı."; exit 0; fi
if [ "$YES" = 0 ]; then
    read -r -p "Devam edilsin mi? (etiket itilir, Release yayımlanır) [y/N] " ans </dev/tty || ans=""
    [ "$ans" = "y" ] || [ "$ans" = "Y" ] || die "iptal edildi"
fi

if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
    git tag -a "$TAG" -m "sshpocket $VERSION"
fi
git push origin "$TAG"

scripts/release.sh
[ -f "$ARCHIVE" ] || die "release.sh paket üretmedi"
[ -f "$ARCHIVE.sig" ] || die "release.sh imza üretmedi"

gh release create "$TAG" "$ARCHIVE" "$ARCHIVE.sig" --title "$TAG" --notes-file "$NOTES" --verify-tag

# Yüklenenlerin özeti yerelle AYNI olmalı
REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
REMOTE=$(gh api "repos/$REPO/releases/tags/$TAG" --jq '.assets[] | "\(.name) \(.digest)"') || die "Release varlıkları okunamadı"
for f in "$ARCHIVE" "$ARCHIVE.sig"; do
    want="sha256:$(SUMOF "$f")"
    got=$(printf '%s\n' "$REMOTE" | awk -v n="$(basename "$f")" '$1 == n {print $2}')
    [ -n "$got" ] || die "Release'te $(basename "$f") yok"
    [ "$got" = "$want" ] || die "$(basename "$f") özeti uyuşmuyor: Release=$got yerel=$want"
done
echo ""
echo "✅ $TAG yayımlandı ve yüklenen dosyaların özeti yerelle aynı."
echo "Arşiv SHA-256: $(SUMOF "$ARCHIVE")"
echo "Sıradaki: USB'yi menüden güncelleyin, sonra doğrulayın."
