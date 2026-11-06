#!/usr/bin/env bash
# ComponentsIndex.sh - İmzalı paket indeksi istemcisi (docs/package-index.md, 3.7.0)
#   ./start.sh --components update          indeksi alır, doğrular, "yükseltilebilir" listesini gösterir (kurmaz)
#   ./start.sh --components upgrade [--yes] [paket…]   yükseltilebilir paketleri onayla kurar
#   ./start.sh --components list            kilit ↔ kurulu ↔ (önbellekteki) indeks
#
# İlkeler: sessiz otomatik kurulum yok; fail-closed (indeks hatasında kurulu paketler çalışır, hiçbir dosya değişmez);
# indeks yeni güven üretmez (paket imzacısı KİLİTTEN gelir, kilitte olmayan paket yok sayılır); geri sürüm yok.
# Test geçersiz kılmaları: comp_index_download (işlev), SSHPOCKET_INDEX_URL, SSHPOCKET_INDEX_SIGNERS, SSHPOCKET_INDEX_NOW (epoch).
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsCLI.sh (_comp_confirm, comp_status)

COMP_INDEX_NAMESPACE="sshpocket-index"
COMP_INDEX_PARSED=""
COMP_INDEX_SERIAL=""

comp_index_signers_file() { printf '%s' "${SSHPOCKET_INDEX_SIGNERS:-${SISTEM_PATH:-.}/index_signers}"; }
comp_index_url() { printf '%s' "${SSHPOCKET_INDEX_URL:-https://raw.githubusercontent.com/syhgorath/sshpocket-index/main}"; }
comp_index_dir() { printf '%s/.index' "$(comp_dir)"; }
comp_index_now() {
    if [[ "${SSHPOCKET_INDEX_NOW:-}" =~ ^[0-9]+$ ]]; then printf '%s' "$SSHPOCKET_INDEX_NOW"; else date +%s; fi
}
comp_sshpocket_version() {
    local v="${SCRIPT_VERSION:-}"
    [ -n "$v" ] || v=$(tr -d '[:space:]' < "${SISTEM_PATH:-.}/../VERSION" 2>/dev/null)
    printf '%s' "$v"
}

# Özellik açık mı? index_signers'ta en az bir (yorum olmayan) anahtar satırı var mı
comp_index_enabled() {
    local f; f=$(comp_index_signers_file)
    [ -f "$f" ] && grep -qE '^[^#[:space:]]' "$f" 2>/dev/null
}

# comp_index_download <url> <hedef> <azami-bayt>: yalnızca HTTPS; testte işlev değiştirilebilir
comp_index_download() {
    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 --max-time 15 --max-filesize "$3" -o "$2" "$1"
}

# comp_index_verify_sig <index.json> <imza>
comp_index_verify_sig() {
    ssh-keygen -Y verify -f "$(comp_index_signers_file)" -I "$COMP_INDEX_NAMESPACE" -n "$COMP_INDEX_NAMESPACE" -s "$2" < "$1" >/dev/null 2>&1
}

# comp_index_state_get <serial|sha256> → kayıtlı değer; yok/bozuksa boş
comp_index_state_get() {
    local f v; f="$(comp_index_dir)/state"
    [ -f "$f" ] || return 0
    v=$(sed -n "s/^$1=//p" "$f" 2>/dev/null | head -n 1 | tr -d '[:space:]')
    case "$1" in
        serial) [[ "$v" =~ ^[0-9]+$ ]] && printf '%s' "$v" ;;
        sha256) comp_sha256_valid "$v" && printf '%s' "$v" ;;
    esac
    return 0
}

# comp_index_parse <index.json> → COMP_INDEX_PARSED, COMP_INDEX_SERIAL; hata iletisi stderr
comp_index_parse() {
    local py out
    py="${SISTEM_PATH:-.}/Helpers/component_index.py"
    command -v python3 >/dev/null 2>&1 || { echo "python3 gerekli (indeks okuma)" >&2; return 1; }
    [ -f "$py" ] || { echo "indeks okuyucu bulunamadı: $py" >&2; return 1; }
    out=$(python3 -I "$py" "$1" "$(comp_index_now)") || return 1
    COMP_INDEX_PARSED="$out"
    COMP_INDEX_SERIAL=$(printf '%s\n' "$out" | awk -F'|' '$1=="meta" {print $2; exit}')
    [[ "$COMP_INDEX_SERIAL" =~ ^[0-9]+$ ]]
}

# comp_index_refresh → 0 kabul edildi (COMP_INDEX_PARSED dolu), 1 hata (fail-closed), 2 özellik kapalı
# Sıra (docs/package-index.md §6): indir → imza → ayrıştır/şema/süre → seri → durumu ilerlet
comp_index_refresh() {
    local tmp base sha st_serial st_sha dir rc
    COMP_INDEX_PARSED=""; COMP_INDEX_SERIAL=""
    if ! comp_index_enabled; then echo "ℹ️  paket indeksi kapalı (sistem/index_signers boş)."; return 2; fi
    base=$(comp_index_url)
    tmp=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-index.XXXXXX") || return 1
    if ! comp_index_download "$base/index.json" "$tmp/index.json" 1048576 \
       || ! comp_index_download "$base/index.json.sig" "$tmp/index.json.sig" 4096; then
        echo "indeks alınamadı: $base" >&2; rm -r -- "${tmp:?}"; return 1
    fi
    if ! comp_index_verify_sig "$tmp/index.json" "$tmp/index.json.sig"; then
        echo "indeks imzası geçersiz; güncelleme yapılmadı" >&2; rm -r -- "${tmp:?}"; return 1
    fi
    if ! comp_index_parse "$tmp/index.json"; then
        echo "indeks reddedildi; güncelleme yapılmadı" >&2; rm -r -- "${tmp:?}"; return 1
    fi
    sha=$(comp_sha256_of "$tmp/index.json")
    st_serial=$(comp_index_state_get serial); st_sha=$(comp_index_state_get sha256)
    if [ -n "$st_serial" ]; then
        if [ "$COMP_INDEX_SERIAL" -lt "$st_serial" ]; then
            echo "eski indeks (seri $COMP_INDEX_SERIAL < $st_serial): geri alma saldırısı olabilir; reddedildi" >&2
            COMP_INDEX_PARSED=""; rm -r -- "${tmp:?}"; return 1
        fi
        if [ "$COMP_INDEX_SERIAL" -eq "$st_serial" ] && [ -n "$st_sha" ] && [ "$sha" != "$st_sha" ]; then
            echo "aynı seri numarası (seri $st_serial) iki farklı içerikle yayımlanmış: reddedildi" >&2
            COMP_INDEX_PARSED=""; rm -r -- "${tmp:?}"; return 1
        fi
    fi
    # kabul: durum ve önbellek ATOMİK yazılır (paket kurulumunun başarısı bundan bağımsızdır)
    dir=$(comp_index_dir); rc=0
    mkdir -p "$dir" \
        && cp "$tmp/index.json" "$dir/index.json.tmp" && cp "$tmp/index.json.sig" "$dir/index.json.sig.tmp" \
        && printf 'serial=%s\nsha256=%s\n' "$COMP_INDEX_SERIAL" "$sha" > "$dir/state.tmp" \
        && mv -f "$dir/index.json.tmp" "$dir/index.json" && mv -f "$dir/index.json.sig.tmp" "$dir/index.json.sig" \
        && mv -f "$dir/state.tmp" "$dir/state" || rc=1
    rm -r -- "${tmp:?}"
    if [ "$rc" -ne 0 ]; then echo "indeks durumu yazılamadı ($dir); güncelleme yapılmadı" >&2; COMP_INDEX_PARSED=""; return 1; fi
    return 0
}

# comp_index_load_cached → son kabul edilen kopyayı yeniden doğrular (imza + süre + durumla aynı seri); ağ yok.
# 0 tamam (COMP_INDEX_PARSED dolu), 1 yok/geçersiz (neden stderr'de; kopya yoksa sessiz)
comp_index_load_cached() {
    local dir st_serial
    COMP_INDEX_PARSED=""; COMP_INDEX_SERIAL=""
    comp_index_enabled || return 1
    dir=$(comp_index_dir)
    [ -f "$dir/index.json" ] && [ -f "$dir/index.json.sig" ] || return 1
    comp_index_verify_sig "$dir/index.json" "$dir/index.json.sig" || { echo "önbellekteki indeksin imzası geçersiz" >&2; return 1; }
    comp_index_parse "$dir/index.json" || return 1
    st_serial=$(comp_index_state_get serial)
    if [ "$COMP_INDEX_SERIAL" != "$st_serial" ]; then echo "önbellekteki indeks durumla uyuşmuyor" >&2; COMP_INDEX_PARSED=""; return 1; fi
    return 0
}

# _comp_index_field <id> <alan-no> → ayrıştırılmış pkg satırından alan (2=id 3=repo 4=sürüm 5=asset 6=sha 7=requires 8=platformlar 9=yanked)
_comp_index_field() {
    printf '%s\n' "$COMP_INDEX_PARSED" | awk -F'|' -v id="$1" -v n="$2" '$1=="pkg" && $2==id {print $n; exit}'
}

# comp_index_eval <id> → "<karar>|<ayrıntı>"
#   unknown (indekste yok/kilitte yok) | yanked | needs-sshpocket|<sürüm> | other-repo | below-floor|<sürüm> | conflict | upgrade|<sürüm> | current
comp_index_eval() {
    local id="$1" iver irepo isha ireq yanked lockv lockrepo locksha have me
    iver=$(_comp_index_field "$id" 4)
    if [ -z "$iver" ] || ! comp_known "$id"; then echo "unknown|"; return 0; fi
    irepo=$(_comp_index_field "$id" 3); isha=$(_comp_index_field "$id" 6)
    ireq=$(_comp_index_field "$id" 7); yanked=$(_comp_index_field "$id" 9)
    lockv=$(comp_get "$id" version); lockrepo=$(comp_get "$id" repo); locksha=$(comp_get "$id" sha256)
    have=$(comp_installed_version "$id"); me=$(comp_sshpocket_version)
    if [ "$yanked" = "1" ]; then echo "yanked|$iver"; return 0; fi
    if [ -n "$me" ] && comp_version_valid "$me" && [ "$(comp_version_cmp "$ireq" "$me")" = "1" ]; then echo "needs-sshpocket|$ireq"; return 0; fi
    if [ "$irepo" != "$lockrepo" ]; then echo "other-repo|$irepo"; return 0; fi
    if [ "$(comp_version_cmp "$iver" "$lockv")" = "-1" ]; then echo "below-floor|$iver"; return 0; fi
    if [ "$iver" = "$lockv" ] && [ "$isha" != "$locksha" ]; then echo "conflict|$iver"; return 0; fi
    if [ -z "$have" ] || [ "$(comp_version_cmp "$iver" "$have")" = "1" ]; then echo "upgrade|$iver"; return 0; fi
    echo "current|$iver"
}

# comp_index_verdict_text <karar> <ayrıntı> <id> → insan okunur satır
comp_index_verdict_text() {
    case "$1" in
        upgrade) printf '⬆️  yükseltilebilir → %s' "$2" ;;
        current) printf '✅ güncel' ;;
        yanked) printf '🚫 indeks %s sürümünü önermiyor (yanked)' "$2" ;;
        needs-sshpocket) printf '⏸️  sshpocket %s gerekli' "$2" ;;
        other-repo) printf '❌ indeks başka depo gösteriyor (%s); yok sayıldı' "$2" ;;
        below-floor) printf '⚠️  indeks sürümü (%s) kilit tabanının altında; yok sayıldı' "$2" ;;
        conflict) printf '❌ indeks ve kilit aynı sürümde çelişiyor (%s); yükseltilmez' "$2" ;;
        *) printf '–' ;;
    esac
}

# comp_index_report → paket başına satır yazar; "yükseltilebilir" kimliklerini COMP_INDEX_UPGRADABLE'a koyar
COMP_INDEX_UPGRADABLE=""
comp_index_report() {
    local id v d have
    COMP_INDEX_UPGRADABLE=""
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        v=$(comp_index_eval "$id"); d="${v#*|}"; v="${v%%|*}"
        [ "$v" != "unknown" ] || continue
        have=$(comp_installed_version "$id")
        printf '  %-20s kurulu %-8s %s\n' "$id" "${have:--}" "$(comp_index_verdict_text "$v" "$d")"
        [ "$v" != "upgrade" ] || COMP_INDEX_UPGRADABLE="$COMP_INDEX_UPGRADABLE$id"$'\n'
    done < <(printf '%s\n' "$COMP_INDEX_PARSED" | awk -F'|' '$1=="pkg" {print $2}')
    return 0
}

# comp_index_install <id> → indeksin sürümünü indirir; sha256 (indeks), imza (kilit imzacısı), geri sürüm yok, arşiv denetimi
comp_index_install() {
    local id="$1" repo ver asset sha work url rc
    ver=$(_comp_index_field "$id" 4); repo=$(_comp_index_field "$id" 3)
    asset=$(_comp_index_field "$id" 5); sha=$(_comp_index_field "$id" 6)
    [ -n "$ver" ] || { echo "indekste yok: $id" >&2; return 1; }
    url="https://github.com/${repo}/releases/download/v${ver}/${asset}"
    work=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-comp.XXXXXX") || return 1
    if comp_download "$url" "$work/pkg.tar.gz" && comp_download "$url.sig" "$work/pkg.tar.gz.sig"; then
        comp_install_archive "$id" "$ver" "$work/pkg.tar.gz" "$work/pkg.tar.gz.sig" "$sha"; rc=$?
    else
        echo "indirme başarısız: $url" >&2; rc=1
    fi
    rm -r -- "${work:?}"
    return "$rc"
}

comp_cli_update() {
    local rc
    comp_lock_validate || return 1
    comp_index_refresh; rc=$?
    [ "$rc" -ne 2 ] || return 0
    [ "$rc" -eq 0 ] || return 1
    echo "Paket indeksi (seri $COMP_INDEX_SERIAL, geçerlilik: $(printf '%s\n' "$COMP_INDEX_PARSED" | awk -F'|' '$1=="meta" {print $4; exit}'))"
    comp_index_report
    if [ -n "$COMP_INDEX_UPGRADABLE" ]; then echo ""; echo "Yükseltmek için: ./start.sh --components upgrade"; fi
    return 0
}

# comp_cli_upgrade [--yes] [paket…] → 0 hata yok, 1 en az bir ret/hata
comp_cli_upgrade() {
    local yes=0 id v d rc ok=0 skipped=0 failed=0 oldv newv a
    local -a want=() reported=()
    for a in "$@"; do
        case "$a" in
            --yes) yes=1 ;;
            -*) echo "kullanım: --components upgrade [--yes] [paket…]" >&2; return 2 ;;
            *) comp_id_valid "$a" || { echo "geçersiz paket kimliği: $a" >&2; return 2; }; want+=("$a") ;;
        esac
    done
    comp_lock_validate || return 1
    comp_index_refresh; rc=$?
    [ "$rc" -ne 2 ] || return 0
    [ "$rc" -eq 0 ] || return 1
    comp_index_report
    if [ -z "$COMP_INDEX_UPGRADABLE" ]; then echo "Yükseltilebilir paket yok."; return 0; fi
    while IFS= read -r id <&3; do
        [ -n "$id" ] || continue
        if [ "${#want[@]}" -gt 0 ]; then
            case " ${want[*]} " in *" $id "*) ;; *) continue ;; esac
        fi
        v=$(comp_index_eval "$id"); d="${v#*|}"
        oldv=$(comp_installed_version "$id")
        if [ "$yes" -eq 0 ] && ! _comp_confirm "$id ${oldv:-kurulu değil} → $d indirilip imzası doğrulanarak kurulsun mu? (kaynak: $(_comp_index_field "$id" 3))"; then
            echo "Atlandı: $id"; skipped=$((skipped + 1)); continue
        fi
        if comp_index_install "$id"; then
            newv=$(comp_installed_version "$id")
            echo "✅ $id $newv"; ok=$((ok + 1))
            [ -z "$oldv" ] || [ "$oldv" = "$newv" ] || reported+=("$id")
        else
            echo "⚠️  $id kurulamadı (eski sürüm olduğu gibi duruyor)"; failed=$((failed + 1))
        fi
    done 3<<< "$COMP_INDEX_UPGRADABLE"
    if declare -F comp_report_pkg >/dev/null; then
        for id in ${reported[@]+"${reported[@]}"}; do echo ""; comp_report_pkg "$id" "$(comp_report_prev_version "$id")" || true; done
    fi
    echo "yükseltilen: $ok, atlanan: $skipped, hata: $failed"
    [ "$failed" -eq 0 ] && [ "$skipped" -eq 0 ]
}
