#!/usr/bin/env bats
# Denetim puanı okuyucu (component_scores.py) ve menü satırı (_comp_menu_score_lines). Sahte summary.json dosyaları, ağ yok.
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    PY="$ROOT/sistem/Helpers/component_scores.py"
    T="$BATS_TEST_TMPDIR"; ST="$T/state"
    D="$ST/pkg/web1/out/web1"; mkdir -p "$D"
}

# mk <alt-klasör> <json>
mk() { mkdir -p "$D/$1"; printf '%s' "$2" > "$D/$1/summary.json"; }

@test "puan: en yeni önce, araç başına son iki; eski biçim hardening_index Lynis sayılır" {
    mk 20260101-000000 '{"date":"2026-01-01T00:00:00Z","hardening_index":50}'
    mk 20260102-000000 '{"date":"2026-01-02T00:00:00Z","hardening_index":61}'
    mk 20260103-000000 '{"date":"2026-01-03T00:00:00Z","hardening_index":83}'
    run python3 -I "$PY" "$ST" pkg web1
    [ "$status" -eq 0 ]
    [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = "2" ]
    [[ "$(printf '%s\n' "$output" | sed -n 1p)" == "Lynis"$'\t'*$'\t'"83" ]] || false
    [[ "$(printf '%s\n' "$output" | sed -n 2p)" == "Lynis"$'\t'*$'\t'"61" ]] || false
}

@test "puan: ortak biçim (tool + score) ayrı araç olarak listelenir; araçlar birbirinin 'önceki'si olmaz" {
    mk a '{"tool":"Lynis","date":"2026-01-03T00:00:00Z","score":80}'
    mk b '{"tool":"OpenSCAP","date":"2026-01-04T00:00:00Z","score":91.5}'
    run python3 -I "$PY" "$ST" pkg web1
    [ "$status" -eq 0 ]
    [[ "$output" == *"Lynis"$'\t'*$'\t'"80"* ]] || false
    [[ "$output" == *"OpenSCAP"$'\t'*$'\t'"91.5"* ]] || false
    [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = "2" ]
}

@test "puan: bozuk JSON, puansız, sayı olmayan/sınır dışı değer ve sembolik bağ yok sayılır; araç adı kısıtlanır" {
    mk bozuk '{bu json degil'
    mk puansiz '{"date":"2026-01-01T00:00:00Z","warnings":2}'
    mk metin '{"score":"yüksek"}'
    mk sinir '{"score":99999}'
    mk mantiksal '{"score":true}'
    mk kotuad '{"tool":"Lynis; rm -rf /","score":70,"date":"2026-01-05T00:00:00Z"}'
    run python3 -I "$PY" "$ST" pkg web1
    [ "$status" -eq 0 ]
    [[ "$output" != *"rm -rf"* ]] || false
    [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = "1" ]
    [[ "$output" == "denetim"$'\t'*$'\t'"70" ]] || false
}

@test "puan: veri ya da klasör yoksa çıktı boş ve çıkış 0; geçersiz paket/sunucu adı reddedilir" {
    run python3 -I "$PY" "$ST" pkg yok; [ "$status" -eq 0 ]; [ -z "$output" ]
    run python3 -I "$PY" "$ST" "../x" web1; [ "$status" -eq 2 ]
    run python3 -I "$PY" "$ST" pkg "a/b";   [ "$status" -eq 2 ]
}

@test "menü satırı: son puan, yaş ve bir öncekiyle fark; veri yoksa hiçbir şey yazmaz" {
    source "$H/Components.sh"; source "$H/ComponentsGates.sh"; source "$H/ComponentsMenu.sh"
    export SSHPOCKET_STATE_DIR="$ST"
    now=$(date -u +%s); old=$(date -u -r $((now - 7200)) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$((now - 7200))" +%Y-%m-%dT%H:%M:%SZ)
    new=$(date -u -r $((now - 60)) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$((now - 60))" +%Y-%m-%dT%H:%M:%SZ)
    mk a "{\"date\":\"$old\",\"hardening_index\":61}"
    mk b "{\"date\":\"$new\",\"hardening_index\":83}"
    run _comp_menu_score_lines pkg web1
    [ "$status" -eq 0 ]
    [[ "$output" == *"📊 Lynis: 83 (1 dk önce) · önceki 61 (+22)"* ]] || false
    run _comp_menu_score_lines pkg yok; [ -z "$output" ]
}

@test "menü satırı: tek kayıt varsa yalnızca puan; puan düşmüşse eksi fark" {
    source "$H/Components.sh"; source "$H/ComponentsGates.sh"; source "$H/ComponentsMenu.sh"
    export SSHPOCKET_STATE_DIR="$ST"
    mk a '{"date":"2026-01-01T00:00:00Z","hardening_index":70}'
    run _comp_menu_score_lines pkg web1; [[ "$output" == *"Lynis: 70 ("* && "$output" != *"önceki"* ]] || false
    mk b '{"date":"2026-01-02T00:00:00Z","hardening_index":65}'
    run _comp_menu_score_lines pkg web1; [[ "$output" == *"Lynis: 65 "*"önceki 70 (-5)"* ]] || false
}
