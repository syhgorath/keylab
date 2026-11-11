#!/usr/bin/env bash
# ComponentsInstall.sh - Paketin güvenli kurulumu (docs/package-standard.md §3, Aşama 2.1)
#
# components/<id>/<sürüm>/        kurulu içerik (imzası doğrulanmış)
# components/<id>/<sürüm>.files   kurulumdaki dosya özetleri (her çalıştırmadan önce yeniden doğrulanır)
# components/<id>/current         etkin sürüm (düz metin; exFAT'ta symlink yok)
# Eski sürümler kalır. Geri sürüme düşülmez. Kullanıcı verisi içermez.
# Gerektirir: Components.sh

comp_dir() {
    if [ -n "${SSHPOCKET_COMPONENTS_DIR:-}" ]; then
        printf '%s' "$SSHPOCKET_COMPONENTS_DIR"
    else
        printf '%s' "${SISTEM_PATH:-.}/../components"
    fi
}

# İki sürümü karşılaştırır; 1 (a>b), 0 (eşit), -1 (a<b)
comp_version_cmp() {
    local -a a b
    local i
    IFS=. read -r -a a <<< "$1"
    IFS=. read -r -a b <<< "$2"
    for i in 0 1 2; do
        if [ "${a[$i]}" -gt "${b[$i]}" ]; then echo 1; return 0; fi
        if [ "${a[$i]}" -lt "${b[$i]}" ]; then echo -1; return 0; fi
    done
    echo 0
}

# HTTPS indirme (yönlendirmelerde de yalnızca HTTPS, boyut sınırlı); testte işlev değiştirilebilir
comp_download() {
    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 \
        --max-time 180 --max-filesize 52428800 -o "$2" "$1"
}

# Kurulu (etkin) sürüm; yoksa boş
comp_installed_version() {
    local f v
    comp_id_valid "$1" || return 0
    f="$(comp_dir)/$1/current"
    [ -f "$f" ] || return 0
    v=$(head -n 1 "$f" 2>/dev/null | tr -d '[:space:]')
    comp_version_valid "$v" && printf '%s' "$v"
    return 0
}

# Arşiv içeriğini açmadan önce denetler. Parametreler: arşiv, kök dizin adı (<id>-<sürüm>)
comp_check_archive() {
    local archive="$1" prefix="$2" names line modes
    names=$(tar -tzf "$archive" 2>/dev/null) || { echo "arşiv okunamadı" >&2; return 1; }
    [ -n "$names" ] || { echo "arşiv boş" >&2; return 1; }
    while IFS= read -r line; do
        case "$line" in
            /*)  echo "mutlak yol: $line" >&2; return 1 ;;
            *..*) echo "şüpheli yol: $line" >&2; return 1 ;;
            "$prefix"|"$prefix"/*) ;;
            *)   echo "beklenmeyen üst dizin: $line" >&2; return 1 ;;
        esac
    done <<< "$names"
    # yalnızca düz dosya ve dizin: symlink, hardlink, aygıt, fifo reddedilir
    modes=$(tar -tvzf "$archive" 2>/dev/null | cut -c1 | sort -u | tr -d '\n')
    case "$modes" in
        ''|-|d|-d|d-) ;;
        *) echo "arşivde düz dosya/dizin dışında öğe var ($modes)" >&2; return 1 ;;
    esac
    return 0
}

# Bildirimden üst düzey alan okur (python3 -I + stdlib json). Parametreler: dosya, anahtar
comp_manifest_get() {
    command -v python3 >/dev/null 2>&1 || { echo "python3 gerekli (bildirim okuma)" >&2; return 2; }
    python3 -I -c '
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    v = d.get(sys.argv[2])
except Exception:
    sys.exit(1)
if isinstance(v, (str, int)) and not isinstance(v, bool):
    print(v)
else:
    sys.exit(1)
' "$1" "$2"
}

# Dizindeki dosyaların özet listesi (yola göre sıralı): "<sha256>  <göreli yol>"
_comp_digest_tree() {
    local dir="$1" f
    ( cd "$dir" || exit 1
      find . -type f ! -name '.DS_Store' | LC_ALL=C sort | while IFS= read -r f; do
          printf '%s  %s\n' "$(comp_sha256_of "$f")" "$f"
      done )
}

# Kurulu sürümün dosyaları kurulumdaki gibi mi? Parametreler: id, sürüm
comp_verify_installed() {
    local id="$1" ver="$2" base cur
    base="$(comp_dir)/$id"
    if [ ! -d "$base/$ver" ] || [ ! -f "$base/$ver.files" ]; then
        echo "kurulu paket bulunamadı: $id $ver" >&2
        return 1
    fi
    cur=$(_comp_digest_tree "$base/$ver") || return 1
    if [ "$cur" != "$(cat "$base/$ver.files")" ]; then
        echo "kurulu paketin dosyaları değişmiş: $id $ver" >&2
        return 1
    fi
}

# Arşivi kurar. Parametreler: id, sürüm, arşiv, imza [beklenen-sha256: doğrulanmış indeksten]
# Sıra: sürüm kilitle uyumlu mu → imza+özet → geri sürüm yok → arşiv denetimi → aç → bildirim kimliği/sürümü → yerleştir → current
# Beklenen özet yoksa sürüm TAM kilit sürümü olmalı. Varsa sürüm ≥ kilit sürümü; kilit sürümüne eşitse özet kilitteki de olmalı.
comp_install_archive() {
    local id="$1" ver="$2" archive="$3" sig="$4" expected="${5:-}" base tmp locked inst cmp rc
    if ! comp_id_valid "$id" || ! comp_version_valid "$ver"; then echo "geçersiz kimlik/sürüm" >&2; return 1; fi
    comp_lock_validate || return 1
    comp_known "$id" || { echo "paket kilit dosyasında yok: $id" >&2; return 1; }
    locked=$(comp_get "$id" version)
    if [ -z "$expected" ]; then
        if [ "$ver" != "$locked" ]; then echo "sürüm kilitle uyuşmuyor ($ver, kilit: $locked)" >&2; return 1; fi
    else
        if [ "$(comp_version_cmp "$ver" "$locked")" = "-1" ]; then echo "sürüm kilit tabanının altında ($ver, kilit: $locked)" >&2; return 1; fi
        if [ "$ver" = "$locked" ] && [ "$expected" != "$(comp_get "$id" sha256)" ]; then echo "özet kilitle uyuşmuyor ($id $ver)" >&2; return 1; fi
    fi
    comp_verify "$id" "$archive" "$sig" "$expected" || return 1

    inst=$(comp_installed_version "$id")
    if [ -n "$inst" ]; then
        cmp=$(comp_version_cmp "$ver" "$inst")
        if [ "$cmp" = "-1" ]; then echo "geri sürüme düşülmez ($inst → $ver)" >&2; return 1; fi
        if [ "$cmp" = "0" ]; then
            comp_verify_installed "$id" "$ver" || return 1
            echo "zaten kurulu: $id $ver"; return 0
        fi
    fi
    comp_check_archive "$archive" "$id-$ver" || return 1

    base="$(comp_dir)/$id"
    mkdir -p "$base" || return 1
    _comp_install_lock "$base" || return 1
    _comp_install_place "$id" "$ver" "$archive" "$base"; rc=$?
    rm -r -- "${base:?}/.install.lock" 2>/dev/null
    return "$rc"
}

# Aynı pakete iki kurulumun aynı anda yerleşmesini engeller (mkdir atomiktir). Kilit sahibi ölmüşse (yarım kalmış) temizlenir.
# Kilit doluysa en fazla ~10 sn beklenir. Başka bir sshpocket (ya da sekme) aynı anda `sync` çalıştırırsa klasörler iç içe girmesin.
_comp_install_lock() {
    local base="$1" lock="$1/.install.lock" other tries=0
    while ! mkdir "$lock" 2>/dev/null; do
        other=$(cat "$lock/pid" 2>/dev/null)
        if [ -n "$other" ] && ! kill -0 "$other" 2>/dev/null; then rm -r -- "${lock:?}" 2>/dev/null; continue; fi
        tries=$((tries + 1))
        if [ "$tries" -gt "${SSHPOCKET_INSTALL_LOCK_TRIES:-20}" ]; then echo "başka bir kurulum sürüyor${other:+ (pid $other)}; bitince yeniden deneyin" >&2; return 1; fi
        sleep 0.5
    done
    printf '%s\n' "$$" > "$lock/pid"
}

# _comp_install_place <id> <sürüm> <arşiv> <paket-klasörü>: KİLİT ALTINDA arşivi açıp yerleştirir (önce yeniden "zaten kurulu mu" bakar)
_comp_install_place() {
    local id="$1" ver="$2" archive="$3" base="$4" tmp inst rc
    inst=$(comp_installed_version "$id")
    if [ "$inst" = "$ver" ]; then
        comp_verify_installed "$id" "$ver" || return 1
        echo "zaten kurulu: $id $ver"; return 0
    fi
    tmp=$(mktemp -d "$base/.tmp.XXXXXX") || return 1
    if ! tar -xzf "$archive" -C "$tmp" 2>/dev/null; then
        rm -r -- "${tmp:?}"; echo "arşiv açılamadı" >&2; return 1
    fi
    if [ ! -d "$tmp/$id-$ver" ] || [ -n "$(find "$tmp" ! -type f ! -type d 2>/dev/null)" ]; then
        rm -r -- "${tmp:?}"; echo "açılan içerik beklenen biçimde değil" >&2; return 1
    fi
    if [ "$(comp_manifest_get "$tmp/$id-$ver/component.json" id)" != "$id" ] \
       || [ "$(comp_manifest_get "$tmp/$id-$ver/component.json" version)" != "$ver" ]; then
        rm -r -- "${tmp:?}"; echo "component.json kimlik/sürüm paketle uyuşmuyor" >&2; return 1
    fi
    if [ -e "$base/$ver" ]; then
        rm -r -- "${tmp:?}"; echo "hedef sürüm klasörü zaten var ($ver); önce elle inceleyin" >&2; return 1
    fi
    chmod -R go-w "$tmp/$id-$ver" 2>/dev/null
    _comp_digest_tree "$tmp/$id-$ver" > "$base/$ver.files.tmp" || { rm -r -- "${tmp:?}"; return 1; }
    mv "$tmp/$id-$ver" "$base/$ver" || { rm -r -- "${tmp:?}"; rm -f "$base/$ver.files.tmp"; return 1; }
    rm -r -- "${tmp:?}"
    mv -f "$base/$ver.files.tmp" "$base/$ver.files"
    printf '%s\n' "$ver" > "$base/current.tmp" && mv -f "$base/current.tmp" "$base/current"
    rc=$?
    if [ "$rc" -ne 0 ]; then
        rm -r -- "${base:?}/${ver:?}"; rm -f "$base/$ver.files" "$base/current.tmp"
        echo "etkin sürüm yazılamadı" >&2; return 1
    fi
    echo "kuruldu: $id $ver"
}

# Kilitteki sürümü indirip kurar (sürümü kilit belirler; "latest" yoktur)
comp_install() {
    local id="$1" repo ver work url rc
    comp_lock_validate || return 1
    comp_known "$id" || { echo "paket kilit dosyasında yok: $id" >&2; return 1; }
    repo=$(comp_get "$id" repo); ver=$(comp_get "$id" version)
    url="https://github.com/${repo}/releases/download/v${ver}/${id}-${ver}.tar.gz"
    work=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-comp.XXXXXX") || return 1
    if comp_download "$url" "$work/pkg.tar.gz" && comp_download "$url.sig" "$work/pkg.tar.gz.sig"; then
        comp_install_archive "$id" "$ver" "$work/pkg.tar.gz" "$work/pkg.tar.gz.sig"; rc=$?
    else
        echo "indirme başarısız: $url" >&2; rc=1
    fi
    rm -r -- "${work:?}"
    return "$rc"
}
