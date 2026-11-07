#!/usr/bin/env bats
load test_helper

@test "VERSION dosyası geçerli bir semver" {
    run cat "$ROOT/VERSION"
    [[ "$output" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}
@test "Main.sh --version VERSION dosyasındaki sürümü yazar" {
    v=$(tr -d '[:space:]' < "$ROOT/VERSION")
    run bash "$ROOT/sistem/Main.sh" --version
    [ "$status" -eq 0 ]
    [ "$output" = "sshpocket $v" ]
}
@test "-V kısa bayrağı da çalışır" {
    run bash "$ROOT/sistem/Main.sh" -V
    [ "$status" -eq 0 ]
    [[ "$output" == sshpocket\ * ]]
}
@test "CHANGELOG'da güncel sürüm için başlık var" {
    v=$(tr -d '[:space:]' < "$ROOT/VERSION")
    grep -q "^## \[$v\] - [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}" "$ROOT/CHANGELOG.md"
}
@test "CHANGELOG'da [Unreleased] bölümü bulunur" {
    grep -q '^## \[Unreleased\]' "$ROOT/CHANGELOG.md"
}
