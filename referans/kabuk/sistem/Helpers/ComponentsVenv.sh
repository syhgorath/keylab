#!/usr/bin/env bash
# ComponentsVenv.sh - Python yorumlayıcı seçimi ve hash'li venv önbelleği (Aşama 2.3)
#
# Venv bir ÖNBELLEKTİR: kontrol makinesinde (Mac) üretilir, taşınmaz, silinebilir, yeniden üretilir.
# Konum: ${SSHPOCKET_CACHE_DIR:-~/.cache/sshpocket}/venv/<paket>-<ortam-kimliği>
# Ortam kimliği = sha256(requirements.txt + python tam sürümü + platform).
# Kurulum YALNIZCA venv'in içine: pip --require-hashes --only-binary :all:; global pip, sudo, brew yok.
# Gerektirir: Components.sh (comp_sha256_of)

COMP_PY_CANDIDATES="python3.14 python3.13 python3.12 python3.11 python3.10 python3"

comp_cache_dir() {
    printf '%s' "${SSHPOCKET_CACHE_DIR:-$HOME/.cache/sshpocket}"
}

# "X.Y.Z" → "X Y" ikilisinin karşılaştırması: 1 (a>b) 0 (eşit) -1 (a<b); yalnızca major.minor
_comp_mm_cmp() {
    local am an bm bn
    am="${1%%.*}"; an="${1#*.}"; an="${an%%.*}"
    bm="${2%%.*}"; bn="${2#*.}"; bn="${bn%%.*}"
    if [ "$am" -gt "$bm" ]; then echo 1; return 0; fi
    if [ "$am" -lt "$bm" ]; then echo -1; return 0; fi
    if [ "$an" -gt "$bn" ]; then echo 1; return 0; fi
    if [ "$an" -lt "$bn" ]; then echo -1; return 0; fi
    echo 0
}

# comp_python_in_range <sürüm X.Y.Z> <aralık ">=3.12" | ">=3.12,<4.0">
comp_python_in_range() {
    local ver="$1" range="$2" lo hi c
    [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    [[ "$range" =~ ^\>=([0-9]+\.[0-9]+)(,\<([0-9]+\.[0-9]+))?$ ]] || return 1
    lo="${BASH_REMATCH[1]}"; hi="${BASH_REMATCH[3]}"
    c=$(_comp_mm_cmp "$ver" "$lo"); [ "$c" != "-1" ] || return 1
    if [ -n "$hi" ]; then
        c=$(_comp_mm_cmp "$ver" "$hi"); [ "$c" = "-1" ] || return 1
    fi
    return 0
}

# comp_python_version <yorumlayıcı> → X.Y.Z ya da boş
comp_python_version() {
    "$1" -c 'import sys; print("%d.%d.%d" % sys.version_info[:3])' 2>/dev/null
}

# Aday yorumlayıcı adları, satır satır (IFS'ten bağımsız). SSHPOCKET_PYTHON_CANDIDATES ile değiştirilebilir.
comp_python_candidates() {
    local IFS=' ' c
    # shellcheck disable=SC2086  # kasıtlı: boşlukla ayrılmış aday listesi
    for c in ${SSHPOCKET_PYTHON_CANDIDATES:-$COMP_PY_CANDIDATES}; do printf '%s\n' "$c"; done
}

# macOS'ta /usr/bin/python3, Command Line Tools kurulu DEĞİLSE gerçek Python değil, "geliştirici araçlarını kurun"
# penceresini açan bir saplamadır. Çalıştırmak o pencereyi tetikler; bu yüzden saplama atlanır.
# (xcode-select yoksa macOS değildir: saplama sayılmaz. Test için yol SSHPOCKET_PY_STUB_PATH ile değiştirilir.)
comp_python_is_stub() {
    [ "$1" = "${SSHPOCKET_PY_STUB_PATH:-/usr/bin/python3}" ] || return 1
    command -v xcode-select >/dev/null 2>&1 || return 1
    ! xcode-select -p >/dev/null 2>&1
}

# Aralığa uyan ilk yorumlayıcının tam yolunu yazar; yoksa dönüş 1
comp_pick_python() {
    local range="$1" cand path ver
    while IFS= read -r cand; do
        path=$(command -v "$cand" 2>/dev/null) || continue
        if comp_python_is_stub "$path"; then continue; fi
        ver=$(comp_python_version "$path")
        [ -n "$ver" ] || continue
        if comp_python_in_range "$ver" "$range"; then printf '%s' "$path"; return 0; fi
    done < <(comp_python_candidates)
    return 1
}

# requirements.txt hash'li mi? Yalnızca "paket==sürüm \" + "--hash=sha256:<64 hex>" devam satırları kabul edilir.
# Her gereksinimin en az bir hash'i olmalı; -r/-e/-c/--index-url gibi başka satırlar REDDEDİLİR.
comp_requirements_hashed() {
    local f="$1" line trimmed have_hash=0 n=0 in_req=0
    local re_req='^[A-Za-z0-9][A-Za-z0-9._-]*(\[[A-Za-z0-9,._-]+\])?==[A-Za-z0-9._+!-]+( *;.*)?( *\\)?$'
    local re_hash='^--hash=sha256:[0-9a-f]{64}( *\\)?$'
    [ -f "$f" ] || { echo "requirements.txt yok" >&2; return 1; }
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        trimmed="${line#"${line%%[![:space:]]*}"}"
        case "$trimmed" in ''|'#'*) continue ;; esac
        if [[ "$trimmed" =~ $re_hash ]]; then
            [ "$in_req" -eq 1 ] || { echo "başıboş hash satırı" >&2; return 1; }
            have_hash=1
        elif [[ "$trimmed" =~ $re_req ]]; then
            if [ "$in_req" -eq 1 ] && [ "$have_hash" -eq 0 ]; then echo "hash'siz gereksinim" >&2; return 1; fi
            in_req=1; have_hash=0; n=$((n + 1))
        else
            echo "kabul edilmeyen satır: $trimmed" >&2; return 1
        fi
    done < "$f"
    if [ "$n" -eq 0 ]; then echo "gereksinim yok" >&2; return 1; fi
    [ "$have_hash" -eq 1 ] || { echo "hash'siz gereksinim" >&2; return 1; }
}

# Ortam kimliği (16 hane): requirements özeti + python tam sürümü + platform
comp_env_id() {
    local req="$1" pyver="$2" s
    s="$(comp_sha256_of "$req")|$pyver|$(uname -s)-$(uname -m)"
    printf '%s' "$s" | { if command -v shasum >/dev/null 2>&1; then shasum -a 256; else sha256sum; fi; } | cut -c1-16
}

comp_venv_dir() { # <paket> <ortam-kimliği>
    printf '%s/venv/%s-%s' "$(comp_cache_dir)" "$1" "$2"
}

# Sağlıklı mı? İşaret dosyası + çalışan python
comp_venv_healthy() {
    local d="$1" envid="$2"
    [ -f "$d/.sshpocket-env" ] && [ "$(cat "$d/.sshpocket-env" 2>/dev/null)" = "$envid" ] || return 1
    [ -x "$d/bin/python" ] && "$d/bin/python" -c 'import sys' >/dev/null 2>&1
}

# pip kurulumu (yalnızca venv içine). Wheelhouse varsa önce çevrimdışı dener, olmazsa internet.
_comp_pip_install() {
    local vpy="$1" req="$2" wh="${3:-}"
    local -a base=("$vpy" -m pip install --no-input --disable-pip-version-check
                   --require-hashes --only-binary :all: -r "$req")
    if [ -n "$wh" ] && [ -d "$wh" ]; then
        if "${base[@]}" --no-index --find-links "$wh" >/dev/null 2>&1; then return 0; fi
    fi
    if [ "${SSHPOCKET_OFFLINE:-0}" = "1" ]; then return 1; fi
    "${base[@]}" >/dev/null 2>&1
}

# comp_venv_lookup <paket> <paket-dizini> <aralık> → sağlıklı venv varsa dizinini yazar; yoksa 1. HİÇBİR ŞEY kurmaz/indirmez.
comp_venv_lookup() {
    local id="$1" pkg="$2" range="$3" py pyver envid dir
    py=$(comp_pick_python "$range") || return 1
    pyver=$(comp_python_version "$py")
    envid=$(comp_env_id "$pkg/requirements.txt" "$pyver")
    dir=$(comp_venv_dir "$id" "$envid")
    comp_venv_healthy "$dir" "$envid" || return 1
    printf '%s' "$dir"
}

# comp_venv_ensure <paket> <paket-dizini> <aralık> [wheelhouse]
# Başarıda venv dizinini yazar. Aynı ortam kimliğinde sağlıklı venv varsa HİÇBİR ŞEY kurulmaz.
# Venv nihai konumunda kurulur (betik başlıkları mutlak yoldur, taşınamaz); işaret dosyası EN SON yazılır,
# yarım kalan kurulum "sağlıksız" sayılır ve silinip yeniden kurulur.
comp_venv_ensure() {
    local id="$1" pkg="$2" range="$3" wh="${4:-}" py pyver envid dir req
    req="$pkg/requirements.txt"
    comp_requirements_hashed "$req" || return 1
    py=$(comp_pick_python "$range") || { echo "uygun Python bulunamadı ($range); ./start.sh --doctor" >&2; return 1; }
    pyver=$(comp_python_version "$py")
    envid=$(comp_env_id "$req" "$pyver")
    dir=$(comp_venv_dir "$id" "$envid")
    if comp_venv_healthy "$dir" "$envid"; then printf '%s' "$dir"; return 0; fi
    if [ -e "$dir" ]; then rm -r -- "${dir:?}"; fi
    mkdir -p "$(dirname "$dir")" || return 1
    if ! "$py" -m venv "$dir" >/dev/null 2>&1; then
        rm -r -- "${dir:?}"; echo "venv oluşturulamadı (python venv/ensurepip eksik olabilir)" >&2; return 1
    fi
    if ! _comp_pip_install "$dir/bin/python" "$req" "$wh"; then
        rm -r -- "${dir:?}"; echo "bağımlılıklar kurulamadı (hash/ağ)" >&2; return 1
    fi
    printf '%s\n' "$envid" > "$dir/.sshpocket-env"
    printf '%s' "$dir"
}
