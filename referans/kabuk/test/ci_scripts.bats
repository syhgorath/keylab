#!/usr/bin/env bats
# CI yardımcı betiği: sabit sürüm ShellCheck kurulumu (özet doğrulaması atlanamaz)
load test_helper

setup() { S="$ROOT/scripts/ci-install-shellcheck.sh"; T="$BATS_TEST_TMPDIR"; }

@test "ci-shellcheck: platform için varlık adı ve 64 haneli sha256 yazdırır" {
    SC_PRINT_ONLY=1 run bash "$S"
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^(linux\.x86_64|darwin\.aarch64|darwin\.x86_64)\ [0-9a-f]{64}$ ]]
}

@test "ci-shellcheck: arşivin özeti uyuşmuyorsa HİÇBİR ŞEY kurulmaz ve hata verir" {
    printf 'sahte arsiv' > "$T/fake.tar.xz"
    mkdir -p "$T/dest"
    SC_LOCAL_ARCHIVE="$T/fake.tar.xz" run bash "$S" "$T/dest"
    [ "$status" -ne 0 ]
    [[ "$output" == *"uyuşmuyor"* ]]
    [ -z "$(ls -A "$T/dest")" ]
}

@test "ci-shellcheck: indirme yalnızca HTTPS ve TLS 1.2+; sertifika denetimi kapatılmaz" {
    grep -q -- "--proto '=https'" "$S"
    grep -q -- '--tlsv1.2' "$S"
    run grep -nE -- '(^|[[:space:]])(-k|--insecure)([[:space:]]|$)' "$S"
    [ "$status" -ne 0 ]
}

@test "ci.yml: shellcheck sabit betikle kurulur (apt/brew'in 'güncel'ine bırakılmaz)" {
    run grep -nE 'apt-get install[^#]*shellcheck|brew install[^#]*shellcheck' "$ROOT/.github/workflows/ci.yml"
    [ "$status" -ne 0 ]
    [ "$(grep -c 'ci-install-shellcheck.sh' "$ROOT/.github/workflows/ci.yml")" -ge 2 ]
}
