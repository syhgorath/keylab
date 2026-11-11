#!/usr/bin/env bash
# Doctor.sh - ./start.sh --doctor: ön koşul denetimi (yalnızca okur; hiçbir şey kurmaz)
# Politika: eksik olanı söyler ve ÇÖZÜM KOMUTUNU gösterir; Homebrew/pip/sudo ÇALIŞTIRMAZ.
# Çıkış kodu: 0 hepsi tamam (uyarılar olabilir), 1 zorunlu bir şey eksik.
# Gerektirir: Components.sh, ComponentsVenv.sh

DOCTOR_FAILS=0

_doc_ok()   { printf '  ✅ %s\n' "$1"; }
_doc_warn() { printf '  ⚠️  %s\n' "$1"; }
_doc_fail() { printf '  ❌ %s\n' "$1"; DOCTOR_FAILS=$((DOCTOR_FAILS + 1)); }
_doc_hint() { printf '     ↳ %s\n' "$1"; }

doctor_run() {
    local range=">=3.12" py ver free_kb cache dir_for_df cand path v found_old="" stub_seen=0 fzf_path
    while [ $# -gt 0 ]; do
        case "$1" in
            --python-range) range="${2:-$range}"; shift 2 ;;
            *) shift ;;
        esac
    done
    DOCTOR_FAILS=0
    echo "sshpocket --doctor"
    echo ""
    echo "Temel araçlar"
    for cmd in ssh ssh-keygen curl tar; do
        if command -v "$cmd" >/dev/null 2>&1; then _doc_ok "$cmd"; else _doc_fail "$cmd bulunamadı"; fi
    done
    if command -v shasum >/dev/null 2>&1 || command -v sha256sum >/dev/null 2>&1; then _doc_ok "sha256 aracı"
    else _doc_fail "shasum/sha256sum bulunamadı"; fi
    if command -v ssh-add >/dev/null 2>&1; then _doc_ok "ssh-add"; else _doc_fail "ssh-add bulunamadı"; fi
    if command -v ssh-copy-id >/dev/null 2>&1; then _doc_ok "ssh-copy-id"
    else _doc_warn "ssh-copy-id yok (key gönderme elle kurulum komutuyla yapılır)"; fi
    if command -v nc >/dev/null 2>&1; then _doc_ok "nc (durum göstergesi)"
    else _doc_warn "nc yok: durum göstergesi ⚪ kalır"; fi
    if fzf_path=$(ui_fzf_bin 2>/dev/null); then _doc_ok "fzf: $fzf_path (isteğe bağlı: arama + açıklama penceresi)"
    else _doc_warn "fzf yok (isteğe bağlı; menü kendi ok tuşlu seçicisiyle çalışır). Kurmak isterseniz: brew install fzf"; fi
    if [ "$(uname -s)" = "Darwin" ]; then
        if command -v security >/dev/null 2>&1; then _doc_ok "security (Keychain)"; else _doc_fail "security (Keychain) bulunamadı"; fi
    else
        _doc_warn "macOS değil: Keychain kullanılamaz (resmî olarak desteklenmiyor)"
    fi

    echo ""
    echo "Python (paketler için; aralık: $range)"
    py=$(comp_pick_python "$range") || py=""
    if [ -n "$py" ]; then
        ver=$(comp_python_version "$py")
        _doc_ok "python $ver ($py)"
        if "$py" -c 'import venv, ensurepip' >/dev/null 2>&1; then _doc_ok "venv + ensurepip"
        else
            _doc_fail "venv/ensurepip eksik"
            _doc_hint "Homebrew Python'unda genelde yerleşiktir; sistem Python'u için: Command Line Tools kurun (xcode-select --install)"
        fi
    else
        while IFS= read -r cand; do
            path=$(command -v "$cand" 2>/dev/null) || continue
            if comp_python_is_stub "$path"; then
                stub_seen=1       # çalıştırılmaz: macOS "araçları kurun" penceresini açar
                continue
            fi
            v=$(comp_python_version "$path")
            if [ -n "$v" ]; then found_old="$found_old $v"; fi
        done < <(comp_python_candidates)
        if [ -n "$found_old" ]; then _doc_fail "uygun Python yok (bulunanlar:$found_old)"
        else _doc_fail "Python bulunamadı"; fi
        if [ "$stub_seen" -eq 1 ]; then
            _doc_hint "$path yalnızca Command Line Tools kurulunca gerçek Python olur; atlandı (açılır pencereyi tetiklemesin diye çalıştırılmadı)"
        fi
        _doc_hint "Homebrew varsa:  brew install python@3.13   (sshpocket bunu kendisi ÇALIŞTIRMAZ)"
        _doc_hint "Homebrew yoksa: https://www.python.org/downloads/ (macOS yükleyicisi)"
    fi

    echo ""
    echo "Disk ve ağ"
    cache=$(comp_cache_dir)
    dir_for_df="$cache"
    while [ ! -d "$dir_for_df" ] && [ "$dir_for_df" != "/" ]; do dir_for_df=$(dirname "$dir_for_df"); done
    free_kb=$(df -k "$dir_for_df" 2>/dev/null | awk 'NR==2 {print $4}')
    if [ -n "$free_kb" ] && [ "$free_kb" -ge 524288 ] 2>/dev/null; then
        _doc_ok "boş alan: $((free_kb / 1024)) MB (önbellek: $cache)"
    else
        _doc_warn "boş alan az ya da okunamadı (önbellek için ~500 MB önerilir): $cache"
    fi
    if [ "${SSHPOCKET_DOCTOR_OFFLINE:-0}" = "1" ]; then
        _doc_warn "ağ denetimi atlandı (SSHPOCKET_DOCTOR_OFFLINE=1)"
    elif curl -sI --proto '=https' --tlsv1.2 --max-time 5 https://github.com >/dev/null 2>&1; then
        _doc_ok "github.com erişilebilir (HTTPS)"
    else
        _doc_warn "github.com'a erişilemedi: güncelleme/paket indirme çalışmaz (USB'de wheelhouse varsa kurulum yine de olabilir)"
    fi

    echo ""
    if [ "$DOCTOR_FAILS" -eq 0 ]; then echo "Sonuç: hazır ✅"; return 0; fi
    echo "Sonuç: $DOCTOR_FAILS zorunlu eksik ❌ (yukarıdaki ↳ önerilere bakın)"
    return 1
}
