#!/usr/bin/env bats
# Depoya yanlışlıkla gizli bilgi/yanlış biçimli güven noktası girmesini engeller.
load test_helper

@test "release_signers: her anahtar satırı beklenen biçimde (yalnızca PUBLIC ed25519)" {
    run grep -v '^[[:space:]]*#' "$ROOT/sistem/release_signers"
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" =~ ^sshpocket-release\ namespaces=\"sshpocket-release\"\ ssh-ed25519\ [A-Za-z0-9+/]+=*$ ]]
    done <<< "$output"
}
@test "release_signers: ssh-keygen satırı gerçek bir anahtar olarak ayrıştırır" {
    key=$(grep -v '^[[:space:]]*#' "$ROOT/sistem/release_signers" | head -n 1 | cut -d' ' -f3,4)
    [ -n "$key" ]
    run bash -c "printf '%s\n' '$key' | ssh-keygen -lf /dev/stdin"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ED25519"* ]]
}
@test "izlenen hiçbir dosyada özel anahtar bloğu yok" {
    command -v git >/dev/null || skip "git yok"
    git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || skip "git deposu değil"
    run git -C "$ROOT" grep -lE 'BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY' -- . ':!test/repo_hygiene.bats'
    [ "$status" -ne 0 ]
}
@test "izlenen dosyalar arasında .env, anahtar veya sır dosyası yok" {
    git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || skip "git deposu değil"
    run bash -c "git -C '$ROOT' ls-files | grep -E '(^|/)\.env$|(^|/)id_(ed25519|rsa|ecdsa)[^.]*$|\.pem$|\.secrets$|sshpocket_release$'"
    [ "$status" -ne 0 ]
}

@test "testlerde başa '!' konmuş iddia yok (bats'ta '! komut' testi düşürmez; 'run ! komut' kullanın)" {
    run bash -c "grep -nE '^[[:space:]]+! ' '$ROOT'/test/*.bats '$ROOT'/test/*.bash | grep -v 'run !' || true"
    [ -z "$output" ]
}

@test "betikler (scripts/, test/e2e/, start*) git'te çalıştırılabilir kipte (100755) izleniyor" {
    git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || skip "git deposu değil"
    # core.fileMode=false olan bir makinede yeni betiklerin 100644 kaydedilmesi unutulabilir; taze klonda 'permission denied' olurdu
    run bash -c "git -C '$ROOT' ls-files -s | awk '\$1 != \"100755\" {print \$4}' | grep -E '^(scripts/[^/]+\\.sh|test/e2e/(lab/)?[^/]+\\.(sh|exp)|start\\.sh|start\\.command)\$'"
    [ "$status" -ne 0 ] || { echo "ÇALIŞTIRILABİLİR DEĞİL: $output"; return 1; }
}

@test "README'lerdeki imza anahtarı ve parmak izi sistem/release_signers ile aynı (anahtar değişince belge unutulmasın)" {
    local key fp
    key=$(grep -v '^#' "$ROOT/sistem/release_signers" | head -n1 | cut -d' ' -f3,4)
    fp=$(printf '%s\n' "$key" | ssh-keygen -lf - | cut -d' ' -f2)
    [ -n "$key" ] && [ -n "$fp" ]
    grep -qF "$key" "$ROOT/README.md"
    grep -qF "$key" "$ROOT/README.en.md"
    grep -qF "$fp" "$ROOT/README.md"
    grep -qF "$fp" "$ROOT/README.en.md"
}
