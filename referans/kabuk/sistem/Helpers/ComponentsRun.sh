#!/usr/bin/env bash
# ComponentsRun.sh - Paket eylemi çalıştırma: girdi üreteçleri (Aşama 3, PR A; VM'e DOKUNMAZ)
# Burada yalnızca dosya/argüman üretilir; hiçbir şey çalıştırılmaz. Kabuk dizgesi ve eval YOKTUR:
# komut bir bash dizisi (COMP_RUN_ARGV) olarak kurulur. docs/package-standard.md §6-§7, §14.
# shellcheck disable=SC2034  # COMP_RUN_ARGV / COMP_RUN_INTERACTIVE çağıran tarafından okunur (dinamik kapsam)
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsManifest.sh (comp_info_get/all, comp_action_get)

COMP_RUN_PY="${SISTEM_PATH:-.}/Helpers/component_run.py"

# comp_run_vars <component.json> <değerler-dosyası> <çıktı.json> [--set ad=değer ...]
# Değerleri doğrular (tür, aralık, required, bilinmeyen ad) ve izin 600'lü değişken dosyasını yazar.
# stdout: "warn=<ad>" satırları + "ok". Değerler stdout'a basılmaz.
comp_run_vars() {
    command -v python3 >/dev/null 2>&1 || { echo "python3 gerekli (değişken doğrulama)" >&2; return 2; }
    [ -f "$COMP_RUN_PY" ] || { echo "değişken doğrulayıcı bulunamadı" >&2; return 2; }
    python3 -I "$COMP_RUN_PY" vars "$@"
}

# _comp_run_inv_safe <değer> → boş değil; satır sonu, tırnak ve ters bölü içermez (INI değeri için güvenli)
_comp_run_inv_safe() {
    [ -n "$1" ] || return 1
    case "$1" in
        *$'\n'*|*$'\r'*|*\'*|*\"*|*\\*) return 1 ;;
    esac
    return 0
}

# comp_run_write_inventory <dosya> <grup> <takma-ad> <host> <kullanıcı> <port> <anahtar-yolu> [become-exe]
# Geçici Ansible inventory (INI) yazar; izin 600, yeni dosya (üzerine yazmaz).
# become-exe (isteğe bağlı): YALNIZCA tam olarak /usr/bin/sudo.ws kabul edilir (Ubuntu'da sudo-rs + parolalı sudo ile Ansible uyumsuzluğunun çözümü);
# başka bir değer reddedilir (envantere keyfi komut yazılamaz).
comp_run_write_inventory() {
    local file="$1" group="$2" alias_="$3" host="$4" user="$5" port="$6" key="$7" become_exe="${8:-}" extra=""
    case "$become_exe" in
        "") ;;
        /usr/bin/sudo.ws) extra=" ansible_become_exe=/usr/bin/sudo.ws" ;;
        *) echo "geçersiz become-exe (yalnızca /usr/bin/sudo.ws)" >&2; return 1 ;;
    esac
    [[ "$group" =~ ^[A-Za-z][A-Za-z0-9_]{0,63}$ ]] || { echo "geçersiz inventory grubu" >&2; return 1; }
    [[ "$alias_" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$ ]] || { echo "geçersiz sunucu adı" >&2; return 1; }
    [[ "$host" =~ ^[A-Za-z0-9][A-Za-z0-9.:_-]{0,252}$ ]] || { echo "geçersiz host/IP" >&2; return 1; }
    [[ "$user" =~ ^[A-Za-z_][A-Za-z0-9_.-]{0,63}$ ]] || { echo "geçersiz kullanıcı" >&2; return 1; }
    if ! [[ "$port" =~ ^[0-9]{1,5}$ ]] || [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then echo "geçersiz port" >&2; return 1; fi
    if [[ "$key" != /* ]] || ! _comp_run_inv_safe "$key"; then echo "anahtar yolu mutlak ve güvenli olmalı" >&2; return 1; fi
    [ ! -e "$file" ] || { echo "inventory dosyası zaten var" >&2; return 1; }
    ( umask 077
      set -C
      printf '[%s]\n%s ansible_host=%s ansible_user=%s ansible_port=%s ansible_private_key_file=%s%s\n' \
          "$group" "$alias_" "$host" "$user" "$port" "'$key'" "$extra" > "$file" ) || { echo "inventory yazılamadı" >&2; return 1; }
}

# comp_run_build_argv <bilgi> <eylem> <inventory> <takma-ad> <değişken-dosyası>
# Sonuç: COMP_RUN_ARGV (dizi), COMP_RUN_INTERACTIVE (0|1). Sıra: cmd sözcükleri, -i, --limit, -e @dosya, invoke.append.
# -i/--limit/-e/--extra-vars ÇALIŞTIRICIYA aittir: pakette cmd içinde geçerse paket reddedilir;
# invoke.append içinde geçerse (bakınız §14) atlanır (yinelenmesin).
comp_run_build_argv() {
    local info="$1" action="$2" inv="$3" alias_="$4" vf="$5"
    local cmd w line arg val rest
    local -a words
    COMP_RUN_ARGV=()
    COMP_RUN_INTERACTIVE=0
    [ "$(comp_info_get "$info" runner)" = "ansible" ] || { echo "yalnızca ansible runner destekleniyor" >&2; return 1; }
    comp_info_all "$info" action | grep -qxF -- "$action" || { echo "bilinmeyen eylem: $action" >&2; return 1; }
    if [ -z "$inv" ] || [ -z "$alias_" ] || [ -z "$vf" ]; then echo "inventory, sunucu adı ve değişken dosyası gerekli" >&2; return 1; fi
    cmd=$(comp_action_get "$info" "$action" cmd)
    [ -n "$cmd" ] || { echo "eylemin komutu yok" >&2; return 1; }
    IFS=' ' read -r -a words <<< "$cmd"
    for w in "${words[@]}"; do
        case "$w" in
            -i|--inventory|--limit|-l|-e|--extra-vars|--inventory=*|--limit=*|--extra-vars=*|-i=*|-l=*|-e=*)
                echo "eylem komutu çalıştırıcıya ait bayrağı içeriyor ($w)" >&2; COMP_RUN_ARGV=(); return 1 ;;
        esac
        COMP_RUN_ARGV+=("$w")
    done
    COMP_RUN_ARGV+=(-i "$inv" --limit "$alias_" -e "@$vf")
    [ "$(comp_action_get "$info" "$action" interactive)" = "yes" ] && COMP_RUN_INTERACTIVE=1
    while IFS= read -r line; do
        [ -n "$line" ] || continue
        arg="${line%%|*}"; rest="${line#*|}"; val="${rest%%|*}"; rest="${rest#*|}"
        case "$arg" in
            -i|--inventory|--limit|-l|-e|--extra-vars) continue ;;
        esac
        if [ -n "$val" ]; then
            case "$val" in
                \<*\>) echo "bilinmeyen yer tutucu: $val" >&2; COMP_RUN_ARGV=(); return 1 ;;
            esac
            COMP_RUN_ARGV+=("$arg" "$val")
        else
            COMP_RUN_ARGV+=("$arg")
        fi
        [ "${rest#*|}" = "yes" ] && COMP_RUN_INTERACTIVE=1
    done < <(comp_info_all "$info" invoke)
    # requires_sudo: sudo parolası Ansible'ın kendi isteminden ('BECOME password:') alınır (standart §5). Paket invoke.append'e
    # --ask-become-pass yazmamış olsa da eklenir; yoksa parolalı sudo'da eylem istem çıkmadan "Missing sudo password" ile düşerdi.
    if [ "$(comp_action_get "$info" "$action" sudo)" = "yes" ]; then
        local has_ask=0
        for w in "${COMP_RUN_ARGV[@]}"; do
            if [ "$w" = "--ask-become-pass" ] || [ "$w" = "-K" ]; then has_ask=1; fi
        done
        if [ "$has_ask" = 0 ]; then COMP_RUN_ARGV+=(--ask-become-pass); COMP_RUN_INTERACTIVE=1; fi
    fi
    return 0
}
