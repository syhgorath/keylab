#!/usr/bin/env bats
# scripts/release-publish.sh: geçici git deposu + yerel "origin" + sahte gh + test anahtarı; gerçek GitHub'a hiçbir şey gitmez.
load test_helper

setup() {
    T="$BATS_TEST_TMPDIR"; R="$T/repo"; O="$T/origin.git"; BIN="$T/bin"
    mkdir -p "$R/scripts" "$R/sistem" "$BIN"
    cp "$ROOT/scripts/release.sh" "$ROOT/scripts/release-publish.sh" "$R/scripts/"
    printf '9.9.9\n' > "$R/VERSION"
    printf '# Changelog\n\n## [Unreleased]\n\n## [9.9.9] - 2026-01-01\n- yeni özellik\n- bir düzeltme\n\n## [9.9.8] - 2025-12-01\n- eski\n' > "$R/CHANGELOG.md"
    printf 'kod\n' > "$R/sistem/Main.sh"
    printf 'dist/\n' > "$R/.gitignore"
    ssh-keygen -q -t ed25519 -N "" -f "$T/key"
    printf 'sshpocket-release namespaces="sshpocket-release" %s\n' "$(cut -d' ' -f1,2 "$T/key.pub")" > "$R/sistem/release_signers"
    git init -q --bare "$O"
    git -C "$R" init -q -b main
    git -C "$R" config user.name t; git -C "$R" config user.email t@t
    git -C "$R" add -A; git -C "$R" commit -q -m init
    git -C "$R" remote add origin "$O"; git -C "$R" push -q origin main
    export SSHPOCKET_SIGNING_KEY="$T/key" GH_LOG="$T/gh.log" PATH="$BIN:$PATH"
    : > "$GH_LOG"
    # sahte gh: davranış ortam değişkenleriyle ayarlanır
    cat > "$BIN/gh" <<'STUB'
#!/bin/bash
echo "gh $*" >> "$GH_LOG"
case "$1 $2" in
  "release view") [ "${GH_RELEASE_EXISTS:-0}" = 1 ] && exit 0; exit 1 ;;
  "run list") echo "${GH_CI:-completed success $(git rev-parse HEAD)}" ;;
  "release create") exit 0 ;;
  "repo view") echo "owner/repo" ;;
  "api repos/owner/repo/releases/tags/v9.9.9")
      for f in dist/*; do
          sum=$(shasum -a 256 "$f" | cut -d' ' -f1)
          [ "${GH_BAD_DIGEST:-0}" = 1 ] && sum="0000"
          echo "$(basename "$f") sha256:$sum"
      done ;;
  *) echo "sahte gh: beklenmeyen: $*" >&2; exit 2 ;;
esac
STUB
    chmod +x "$BIN/gh"
}

pub() { run bash "$R/scripts/release-publish.sh" "$@"; }

@test "dry-run: planı gösterir, etiket/paket/Release OLUŞTURMAZ, çıkış 0" {
    pub --dry-run
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sürüm: v9.9.9"* && "$output" == *"[dry-run] hiçbir şey yapılmadı."* ]] || false
    [[ "$output" == *"2 satır"* ]] || false
    run git -C "$R" tag -l; [ -z "$output" ]
    [ ! -d "$R/dist" ]
    run grep -c "release create" "$GH_LOG"; [ "$output" = "0" ]
}

@test "reddeder: main dışında bir dalda" {
    git -C "$R" checkout -q -b baska
    pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"main dalında değilsiniz"* ]] || false
}

@test "reddeder: çalışma alanı kirliyse" {
    echo x >> "$R/sistem/Main.sh"
    pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"temiz değil"* ]] || false
}

@test "reddeder: HEAD origin/main ile aynı değilse (yerelde itilmemiş commit)" {
    echo y >> "$R/sistem/Main.sh"; git -C "$R" commit -qam yerel
    pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"origin/main ile aynı değil"* ]] || false
}

@test "reddeder: CHANGELOG'da sürüm başlığı ya da bölümü yoksa" {
    sed -i.bak 's/## \[9.9.9\]/## [9.9.7]/' "$R/CHANGELOG.md"; rm -f "$R/CHANGELOG.md.bak"
    git -C "$R" commit -qam c; git -C "$R" push -q origin main
    pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"[9.9.9] başlığı yok"* ]] || false
}

@test "reddeder: imza anahtarı yoksa, etiket İTİLMEDEN önce" {
    SSHPOCKET_SIGNING_KEY="$T/yok" pub --yes
    [ "$status" -ne 0 ]; [[ "$output" == *"imza anahtarı bulunamadı"* ]] || false
    run git -C "$O" tag -l; [ -z "$output" ]
}

@test "reddeder: Release zaten varsa" {
    GH_RELEASE_EXISTS=1 pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"Release v9.9.9 zaten var"* ]] || false
}

@test "reddeder: main CI yeşil değilse ya da başka commit'e aitse; --no-ci-check ile geçer" {
    GH_CI="in_progress null abc" pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"CI yeşil değil"* ]] || false
    GH_CI="completed success deadbeef" pub --dry-run; [ "$status" -ne 0 ]
    GH_CI="in_progress null abc" pub --dry-run --no-ci-check; [ "$status" -eq 0 ]
}

@test "reddeder: uzaktaki ya da yerel etiket başka bir commit'i gösteriyorsa" {
    git -C "$R" tag -a v9.9.9 -m eski
    echo z >> "$R/sistem/Main.sh"; git -C "$R" commit -qam sonra; git -C "$R" push -q origin main
    pub --dry-run; [ "$status" -ne 0 ]; [[ "$output" == *"etiketi şu anki commit'i göstermiyor"* ]] || false
}

@test "onay istenir: --yes yoksa ve terminal yoksa iptal edilir, hiçbir şey yapılmaz" {
    pub
    [ "$status" -ne 0 ]; [[ "$output" == *"iptal edildi"* ]] || false
    run git -C "$O" tag -l; [ -z "$output" ]
}

@test "gerçek koşu (--yes): etiket origin'e itilir, imzalı paket üretilir, Release iki dosya ve CHANGELOG notuyla açılır, özet doğrulanır" {
    pub --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"v9.9.9 yayımlandı ve yüklenen dosyaların özeti yerelle aynı"* ]] || false
    [ "$(git -C "$O" rev-list -n1 v9.9.9)" = "$(git -C "$R" rev-parse HEAD)" ]
    [ -f "$R/dist/sshpocket-9.9.9.tar.gz" ] && [ -f "$R/dist/sshpocket-9.9.9.tar.gz.sig" ]
    ssh-keygen -Y verify -f "$R/sistem/release_signers" -I sshpocket-release -n sshpocket-release \
        -s "$R/dist/sshpocket-9.9.9.tar.gz.sig" < "$R/dist/sshpocket-9.9.9.tar.gz" >/dev/null
    create=$(grep "release create" "$GH_LOG")
    [[ "$create" == *"v9.9.9 dist/sshpocket-9.9.9.tar.gz dist/sshpocket-9.9.9.tar.gz.sig"* ]] || false
    [[ "$create" == *"--verify-tag"* && "$create" == *"--notes-file"* ]] || false
}

@test "özet uyuşmazsa hata verir (Release'e yüklenen dosya yerelle farklı)" {
    GH_BAD_DIGEST=1 pub --yes
    [ "$status" -ne 0 ]; [[ "$output" == *"özeti uyuşmuyor"* ]] || false
    [[ "$output" != *"yayımlandı ve yüklenen"* ]] || false
}

@test "yeniden çalıştırma: etiket zaten HEAD'deyse (önceki koşu yarıda kaldı) tekrar etiketlemeden devam eder" {
    git -C "$R" tag -a v9.9.9 -m "sshpocket 9.9.9"
    pub --yes
    [ "$status" -eq 0 ]
    [ "$(git -C "$O" rev-list -n1 v9.9.9)" = "$(git -C "$R" rev-parse HEAD)" ]
}

@test "bilinmeyen bağımsız değişken reddedilir; --help kullanımı yazar" {
    pub --bilinmeyen; [ "$status" -ne 0 ]; [[ "$output" == *"bilinmeyen bağımsız değişken"* ]] || false
    pub --help; [ "$status" -eq 0 ]; [[ "$output" == *"release-publish.sh"* ]] || false
}
