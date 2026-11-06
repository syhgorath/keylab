#!/usr/bin/env bash
# ComponentsCLI.sh - Paketlerin komut satırı/menü yönetimi (Aşama 2.4a)
#   ./start.sh --components list        kilit ↔ kurulu durumu (ağ yok)
#   ./start.sh --components sync [--yes] kilitteki sürümü kur/onar (onay ister)
#   ./start.sh --components update | upgrade [--yes] [paket…]   imzalı indeks: yükseltilebilir listesi / onaylı yükseltme (ComponentsIndex.sh)
#   ./start.sh --components check-new   kilitten daha yeni yayın var mı? (yalnızca bilgi; ağ)
#   ./start.sh --components run <paket> <eylem> <sunucu>   kurulu paketin eylemini sunucuda çalıştırır (ComponentsRunCLI.sh)
#   ./start.sh --components validate <paket-kimliği | dizin | arşiv.tar.gz> [--strict] [--unsigned]   paket SÖZLEŞME uyum denetimi (statik; docs/package-standard.md §12)
#
# `sync` kilit sürümünü kurar (çevrimdışı/ilk kurulum). Kilitten yeni sürüm yalnızca imzalı indeksle (`upgrade`, onaylı) gelir.
# Gerektirir: Components.sh, ComponentsInstall.sh

# HTTPS başlık isteği; Location değerini yazdırır (testte işlev değiştirilebilir)
comp_http_head() {
    curl -sSI --proto '=https' --tlsv1.2 --max-time 15 "$1" 2>/dev/null \
        | tr -d '\r' | awk 'tolower($1)=="location:" {print $2; exit}'
}

# GitHub API: depodaki yayınlar (ön sürümler dahil, en yeni başta). Testte işlev değiştirilebilir.
comp_http_json() {
    curl -sS --proto '=https' --tlsv1.2 --max-time 15 -H 'Accept: application/vnd.github+json' "$1" 2>/dev/null
}

# comp_latest_version <depo> → en son yayın sürümü (v'siz); geçersizse hata
# Önce kararlı "latest" yönlendirmesi; ön sürüm (pre-release) olduğu için "latest" yoksa API'deki en yeni yayına düşer.
comp_latest_version() {
    local repo="$1" loc tag json
    comp_repo_valid "$repo" || return 1
    loc=$(comp_http_head "https://github.com/${repo}/releases/latest")
    if [ -n "$loc" ]; then
        tag="${loc##*/}"
        [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
        [ "$loc" = "https://github.com/${repo}/releases/tag/${tag}" ] || return 1
        printf '%s' "${tag#v}"
        return 0
    fi
    json=$(comp_http_json "https://api.github.com/repos/${repo}/releases?per_page=1") || return 1
    tag=$(printf '%s\n' "$json" | sed -n 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
    [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    printf '%s' "${tag#v}"
}

# comp_status <id> → ok | missing | outdated | ahead | broken
comp_status() {
    local id="$1" want have cmp
    want=$(comp_get "$id" version)
    have=$(comp_installed_version "$id")
    if [ -z "$have" ]; then echo missing; return 0; fi
    cmp=$(comp_version_cmp "$have" "$want")
    case "$cmp" in
        -1) echo outdated; return 0 ;;
        1)  # kilitten yeni (indeksle yükseltilmiş): dosyalar yine de kurulumdaki gibi olmalı
            if comp_verify_installed "$id" "$have" >/dev/null 2>&1; then echo ahead; else echo broken; fi
            return 0 ;;
    esac
    if comp_verify_installed "$id" "$have" >/dev/null 2>&1; then echo ok; else echo broken; fi
}

_comp_confirm() {
    local a
    if ! read -r -p "$1 [y/N] " a; then return 1; fi
    case "$a" in y|Y|e|E) return 0 ;; *) return 1 ;; esac
}

# Bozulmuş kurulumu silmeden kenara alır (<sürüm>.bozuk-<tarih>); current kaldırılır
_comp_quarantine() {
    local id="$1" ver="$2" base stamp
    base="$(comp_dir)/$id"
    stamp=$(date +%Y%m%d-%H%M%S)
    [ -d "$base/$ver" ] && mv "$base/$ver" "$base/$ver.bozuk-$stamp"
    [ -f "$base/$ver.files" ] && mv "$base/$ver.files" "$base/$ver.files.bozuk-$stamp"
    rm -f "$base/current"
}

comp_cli_list() {
    local ids id st want have mark v d idxmsg
    comp_lock_validate || return 1
    # önbellekteki indeks (ağ yok): yeniden doğrulanır; geçersizse yalnızca nedeni yazılır
    COMP_INDEX_PARSED=""
    if declare -F comp_index_load_cached >/dev/null; then
        idxmsg=$(mktemp "${TMPDIR:-/tmp}/sshpocket-idx.XXXXXX") || idxmsg=/dev/null
        if ! comp_index_load_cached 2>"$idxmsg"; then
            COMP_INDEX_PARSED=""
            [ ! -s "$idxmsg" ] || echo "ℹ️  indeks önbelleği kullanılamıyor: $(head -n 1 "$idxmsg")"
        fi
        [ "$idxmsg" = /dev/null ] || rm -f "$idxmsg"
    fi
    ids=$(comp_ids)
    if [ -z "$ids" ]; then echo "Kilit dosyasında paket yok."; return 0; fi
    echo "Paketler (components.lock)"
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        st=$(comp_status "$id")
        want=$(comp_get "$id" version)
        have=$(comp_installed_version "$id")
        case "$st" in
            ok) mark="✅ kurulu" ;;
            missing) mark="⬇️  kurulmamış" ;;
            outdated) mark="⬆️  güncellenmeli" ;;
            ahead) mark="✅ kilitten yeni (indeksle yükseltilmiş)" ;;
            broken) mark="❌ dosyalar değişmiş (onarım gerekir)" ;;
        esac
        printf '  %-20s kilit %-8s kurulu %-8s %s\n' "$id" "$want" "${have:--}" "$mark"
        if [ -n "$COMP_INDEX_PARSED" ]; then
            v=$(comp_index_eval "$id"); d="${v#*|}"; v="${v%%|*}"
            [ "$v" = "unknown" ] || [ "$v" = "current" ] || printf '  %-20s indeks: %s\n' "" "$(comp_index_verdict_text "$v" "$d")"
        fi
    done <<< "$ids"
}

# comp_cli_sync [--yes] → 0 hepsi tamam, 1 en az bir hata/ret
comp_cli_sync() {
    local yes=0 ids id st want rc=0 oldv
    local -a reported=()
    [ "${1:-}" = "--yes" ] && yes=1
    comp_lock_validate || return 1
    ids=$(comp_ids)
    [ -n "$ids" ] || return 0
    while IFS= read -r id <&3; do
        [ -n "$id" ] || continue
        [ -z "${COMP_SYNC_ONLY:-}" ] || [ "$id" = "$COMP_SYNC_ONLY" ] || continue   # menü: tek paketi eşitle
        st=$(comp_status "$id"); want=$(comp_get "$id" version)
        case "$st" in
            ok) continue ;;
            ahead) echo "⚠️  $id: kurulu sürüm kilitten yeni; dokunulmadı."; continue ;;
            missing|outdated)
                if [ "$yes" -eq 0 ] && ! _comp_confirm "$id $want indirilip imzası doğrulanarak kurulsun mu?"; then
                    echo "Atlandı: $id"; rc=1; continue
                fi
                ;;
            broken)
                echo "❌ $id: kurulu dosyalar değişmiş (kurulumdan sonra düzenlenmiş olabilir)."
                if [ "$yes" -eq 0 ] && ! _comp_confirm "Bozuk kurulum kenara alınıp $id $want yeniden kurulsun mu?"; then
                    echo "Atlandı: $id"; rc=1; continue
                fi
                _comp_quarantine "$id" "$(comp_installed_version "$id")" || { rc=1; continue; }
                ;;
        esac
        oldv=$(comp_installed_version "$id")
        if comp_install "$id"; then
            echo "✅ $id $want"
            [ -z "$oldv" ] || [ "$oldv" = "$want" ] || reported+=("$id")      # sürüm değişti: ne değiştiğini aşağıda göster
        else echo "⚠️  $id kurulamadı"; rc=1; fi
    done 3<<< "$ids"
    if declare -F comp_report_pkg >/dev/null; then
        for id in ${reported[@]+"${reported[@]}"}; do echo ""; comp_report_pkg "$id" "$(comp_report_prev_version "$id")" || true; done
    fi
    return "$rc"
}

comp_cli_check_new() {
    local ids id want latest repo
    comp_lock_validate || return 1
    ids=$(comp_ids)
    [ -n "$ids" ] || { echo "Kilit dosyasında paket yok."; return 0; }
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        repo=$(comp_get "$id" repo); want=$(comp_get "$id" version)
        if ! latest=$(comp_latest_version "$repo"); then echo "ℹ️  $id: yayın bilgisi alınamadı"; continue; fi
        if [ "$(comp_version_cmp "$latest" "$want")" = "1" ]; then
            echo "🆕 $id: $latest yayınlanmış (sshpocket'ın kilidi: $want)."
            echo "   Yeni sürüm, yeni bir sshpocket sürümüyle (kilit güncellenince) gelir; şimdi kurulmaz."
        else
            echo "✅ $id: güncel ($want)"
        fi
    done <<< "$ids"
    return 0
}

# comp_cli_validate <kurulu-paket-kimliği | dizin | <id>-<sürüm>.tar.gz> [--strict]
# Statik denetim: ansible/ssh/ağ çalıştırmaz, paketten hiçbir şey çalıştırmaz (component_validate.py).
comp_cli_validate() {
    local target="${1:-}" ver py
    py="${SISTEM_PATH:-.}/Helpers/component_validate.py"
    if [ -z "$target" ] || [ "${target:0:2}" = "--" ]; then
        echo "kullanım: ./start.sh --components validate <kurulu-paket-kimliği | dizin | arşiv.tar.gz> [--strict]" >&2; return 2
    fi
    command -v python3 >/dev/null 2>&1 || { echo "python3 gerekli (paket denetimi)" >&2; return 2; }
    [ -f "$py" ] || { echo "denetleyici bulunamadı: $py" >&2; return 2; }
    # kurulu bir paketin kimliği verildiyse onun etkin sürüm klasörü denetlenir
    if [ ! -e "$target" ] && comp_id_valid "$target"; then
        ver=$(comp_installed_version "$target")
        [ -n "$ver" ] || { echo "kurulu paket yok ve '$target' bir dizin/arşiv değil" >&2; return 2; }
        shift
        python3 -I "$py" "$(comp_dir)/$target/$ver" "$@"
        return $?
    fi
    python3 -I "$py" "$@"
}

# comp_cli_prune [--yes] [--dry-run] → eski sürüm klasörlerini siler. KALANLAR: etkin sürüm (current), bir önceki sürüm (geri dönüş
# için) ve kilitteki sürüm (kurulu ise). Yalnızca <X.Y.Z> klasörü + <X.Y.Z>.files silinir; .bozuk-* (kenara alınmış bozuk kurulum),
# sembolik bağlar ve geçersiz adlar DOKUNULMAZ. current'ı olmayan pakete dokunulmaz. --dry-run: yalnızca listeler.
comp_cli_prune() {
    local yes=0 dry=0 arg d id base cur lockv prev v rm_list n_rm=0 n_keep
    for arg in "$@"; do
        case "$arg" in
            --yes) yes=1 ;;
            --dry-run) dry=1 ;;
            *) echo "kullanım: --components prune [--yes] [--dry-run]" >&2; return 2 ;;
        esac
    done
    local ids_all=""
    while IFS= read -r d; do
        [ -n "$d" ] || continue
        id=$(basename "$d")
        comp_id_valid "$id" || continue
        [ -L "$d" ] && continue
        ids_all="$ids_all$id"$'\n'
    done < <(find "$(comp_dir)" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)
    [ -n "$ids_all" ] || { echo "Kurulu paket yok."; return 0; }

    local removals=""
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        base="$(comp_dir)/$id"
        cur=$(comp_installed_version "$id")
        if [ -z "$cur" ]; then echo "  $id: etkin sürüm (current) yok; dokunulmadı"; continue; fi
        lockv=""
        if [ -f "$(comp_lock_file)" ] && comp_known "$id"; then lockv=$(comp_get "$id" version); fi
        prev=""
        while IFS= read -r d; do
            v=$(basename "$d")
            comp_version_valid "$v" || continue
            [ -L "$d" ] && continue
            [ "$(comp_version_cmp "$v" "$cur")" = "-1" ] || continue
            if [ -z "$prev" ] || [ "$(comp_version_cmp "$v" "$prev")" = "1" ]; then prev="$v"; fi
        done < <(find "$base" -mindepth 1 -maxdepth 1 -type d 2>/dev/null)
        rm_list=""; n_keep=0
        while IFS= read -r d; do
            v=$(basename "$d")
            comp_version_valid "$v" || continue
            [ -L "$d" ] && continue
            if [ "$v" = "$cur" ] || [ "$v" = "$prev" ] || [ "$v" = "$lockv" ]; then
                n_keep=$((n_keep + 1))
            else
                rm_list="$rm_list$v"$'\n'
            fi
        done < <(find "$base" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)
        if [ -z "$rm_list" ]; then
            echo "  $id: temiz (kalan $n_keep sürüm; etkin $cur)"
        else
            echo "  $id: etkin $cur, önceki ${prev:--}, kilit ${lockv:--} KALIR; silinecek: $(printf '%s' "$rm_list" | tr '\n' ' ')"
            while IFS= read -r v; do
                [ -n "$v" ] || continue
                removals="$removals$id $v"$'\n'; n_rm=$((n_rm + 1))
            done <<< "$rm_list"
        fi
    done <<< "$ids_all"
    [ "$n_rm" -gt 0 ] || { echo "Silinecek eski sürüm yok."; return 0; }
    [ "$dry" -eq 0 ] || { echo "(--dry-run: hiçbir şey silinmedi)"; return 0; }
    if [ "$yes" -eq 0 ] && ! _comp_confirm "$n_rm eski sürüm klasörü silinsin mi?"; then
        echo "Atlandı."; return 1
    fi
    while IFS=' ' read -r id v; do
        [ -n "$id" ] || continue
        if ! comp_id_valid "$id" || ! comp_version_valid "$v"; then continue; fi
        base="$(comp_dir)/$id"
        if [ ! -d "$base/$v" ] || [ -L "$base/$v" ]; then continue; fi
        rm -r -- "${base:?}/${v:?}" && rm -f -- "$base/$v.files" && echo "silindi: $id $v"
    done <<< "$removals"
    return 0
}

components_cli() {
    local sub="${1:-list}"
    [ $# -gt 0 ] && shift
    case "$sub" in
        list) comp_cli_list ;;
        sync) comp_cli_sync "$@" ;;
        update) comp_cli_update "$@" ;;
        upgrade) comp_cli_upgrade "$@" ;;
        check-new) comp_cli_check_new ;;
        run) comp_cli_run "$@" ;;
        validate) comp_cli_validate "$@" ;;
        prune) comp_cli_prune "$@" ;;
        report) if [ $# -gt 0 ]; then comp_report_pkg "$@"; else comp_report_all; fi ;;
        *) echo "kullanım: --components list | sync [--yes] | update | upgrade [--yes] [paket…] | check-new | prune [--yes] [--dry-run] | report [paket [eski-sürüm]] | run <paket> <eylem> <sunucu> | validate <paket|dizin|arşiv> [--strict]" >&2; return 2 ;;
    esac
}
