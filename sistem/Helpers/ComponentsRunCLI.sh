#!/usr/bin/env bash
# ComponentsRunCLI.sh - ./start.sh --components run <paket> <eylem> <sunucu>  (Aşama 3, PR D)
# Sunucu bilgisini okur, işletim sistemi profilini SALT OKUNUR bir ssh ile algılar, gerekirse (onayla) Python ortamını kurar
# ve yürütücüyü (ComponentsExec.sh) çağırır. Parola/passphrase istemez: sudo parolasını Ansible kendi isteminden alır.
# Çıkış kodları: 0 başarı · 2 kullanım hatası · 190 çalıştırılmadı (kapı/onay/hazırlık) · aksi halde playbook'un kodu
# shellcheck disable=SC2034  # COMP_CTX_* değişkenlerini ComponentsExec.sh (dinamik kapsam) okur
# Gerektirir: Components*.sh, ComponentsRun.sh, ComponentsGates.sh, ComponentsExec.sh, Servers.sh, Validate.sh, ModuleFiles.sh

# comp_detect_profile <host> <port> <kullanıcı> <anahtar> → "ubuntu-26.04" (ID-VERSION_ID). Uzak komut SABİT ve salt okunurdur.
# Host key kapatılmaz (StrictHostKeyChecking=yes), parola sorulmaz (BatchMode). Çıktı asla source/eval edilmez.
comp_detect_profile() {
    local host="$1" port="$2" user="$3" key="$4" out id ver
    out=$(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 \
              -o PreferredAuthentications=publickey -o PasswordAuthentication=no \
              -p "$port" -i "$key" "$user@$host" 'cat /etc/os-release' 2>"${COMP_DETECT_ERRFILE:-/dev/null}") || return 1
    id=$(printf '%s\n' "$out" | awk -F= '$1=="ID" { gsub(/"/, "", $2); print tolower($2); exit }')
    ver=$(printf '%s\n' "$out" | awk -F= '$1=="VERSION_ID" { gsub(/"/, "", $2); print tolower($2); exit }')
    [[ "$id" =~ ^[a-z0-9][a-z0-9._-]{0,31}$ ]] && [[ "$ver" =~ ^[a-z0-9][a-z0-9._-]{0,31}$ ]] || return 1
    printf '%s-%s' "$id" "$ver"
}

# comp_detect_sudo_ws <host> <port> <kullanıcı> <anahtar> → 0 ise sunucuda /usr/bin/sudo.ws çalıştırılabilir (Ubuntu'da klasik sudo; sudo-rs yanında).
# Uzak komut SABİT ve salt okunurdur; host key kapatılmaz, parola sorulmaz. Çıktı yalnızca tam satır eşleşmesiyle değerlendirilir.
comp_detect_sudo_ws() {
    local host="$1" port="$2" user="$3" key="$4" out
    out=$(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 \
              -o PreferredAuthentications=publickey -o PasswordAuthentication=no \
              -p "$port" -i "$key" "$user@$host" 'test -x /usr/bin/sudo.ws && echo SSHPOCKET_SUDO_WS=yes' 2>/dev/null) || return 1
    printf '%s\n' "$out" | grep -qx 'SSHPOCKET_SUDO_WS=yes'
}

# _comp_fmt_utc <epoch> → HH:MM:SS (UTC); GNU (-d @) ve BSD/macOS (-r) date ile çalışır
_comp_fmt_utc() { date -u -d "@$1" +%H:%M:%S 2>/dev/null || date -u -r "$1" +%H:%M:%S 2>/dev/null || printf '?'; }

# comp_warn_clock_skew <host> <port> <kullanıcı> <anahtar> → sunucu saati yerel saatten 60 sn'den fazla farklıysa uyarır (stderr); ENGELLEMEZ, hep 0 döner.
# Snapshot geri alınınca VM saati geride kalır ve apt "Not live until" ile düşer; TOTP/2FA kodları da saat kaymasında reddedilir.
# Uzak komut SABİT ve salt okunurdur (saat + saat dilimi + NTP durumu); okunamazsa sessiz geçer. Düzeltmeyi KULLANICI yapar (saat sıçrar); burada yalnızca hazır komut gösterilir.
comp_warn_clock_skew() {
    local host="$1" port="$2" user="$3" key="$4" out remote local_now diff min dir tz ntp qkey off line
    out=$(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 \
              -o PreferredAuthentications=publickey -o PasswordAuthentication=no \
              -p "$port" -i "$key" "$user@$host" 'date +%s; timedatectl show -p Timezone -p NTPSynchronized 2>/dev/null' 2>/dev/null) || return 0
    remote=$(printf '%s\n' "$out" | head -n 1)
    [[ "$remote" =~ ^[0-9]{1,12}$ ]] || return 0
    tz=""; ntp=""
    while IFS= read -r line; do
        if [[ "$line" =~ ^Timezone=([A-Za-z0-9_+/-]{1,64})$ ]]; then tz="${BASH_REMATCH[1]}"; fi
        if [[ "$line" =~ ^NTPSynchronized=(yes|no)$ ]]; then ntp="${BASH_REMATCH[1]}"; fi
    done <<< "$out"
    local_now=$(date +%s)
    diff=$(( 10#$remote - local_now ))
    if [ "$diff" -lt 0 ]; then dir="geride"; diff=$(( -diff )); else dir="ileride"; fi
    [ "$diff" -gt 60 ] || return 0
    min=$(( (diff + 30) / 60 ))
    printf -v qkey '%q' "$key"
    {
        echo "  ⚠️  Sunucu saati yaklaşık $min dakika $dir (snapshot geri alındıysa olağandır)."
        echo "      Sunucu: $(_comp_fmt_utc "$((10#$remote))") UTC${tz:+ (saat dilimi: $tz)} · Bu bilgisayar: $(_comp_fmt_utc "$local_now") UTC · Fark: $((diff / 60)) dk $((diff % 60)) sn $dir"
        echo "      apt \"Not live until\" hatası verebilir; audit/apply başarısız olabilir. Sunucuda TOTP/2FA varsa kodlar da reddedilir."
        # fark neredeyse tam saat (en az 30 dk, 30 dk'nın katı ±2 dk) ise saat dilimi karışıklığı olabilir
        off=$(( diff % 1800 ))
        if [ "$diff" -ge 1800 ] && { [ "$off" -lt 120 ] || [ "$off" -gt 1680 ]; }; then
            echo "      Fark tam saat dilimi farkına benziyor (~$(( (diff + 900) / 1800 )) yarım saat): sunucu saati yerel saat olarak ayarlanmış olabilir. 'timedatectl' ile saat dilimini ve RTC ayarını kontrol edin."
        fi
        [ "$ntp" != "no" ] || echo "      NTP eşitlemesi kapalı/başarısız (NTPSynchronized=no): düzeltme kalıcı olmayabilir; chrony kaynaklarını kontrol edin."
        echo "      Düzeltme (hazır komut, sudo parolanızı siz yazarsınız; saat SIÇRAR):"
        echo "        ssh -t -p $port -i $qkey $user@$host 'sudo systemctl restart chrony; sleep 8; sudo chronyc makestep; date -u'"
        echo "      Ya da sunucuda: sudo chronyc makestep"
        echo "      Ya da paket seçeneği: linux-hardening için hardening_time_makestep=true (apply saati düzeltir; saati sıçratır, varsayılan kapalı)."
    } >&2
    return 0
}

# comp_ssh_hint <ssh-hata-dosyası> → bilinen ssh hatalarını açıklar ve ne yapılacağını söyler (stdout; boşsa hiçbir şey yazmaz).
# Hata metni sunucudan gelebilir (banner vb.) → denetim karakterleri temizlenir, kesilir; kabuğa/eval'e HİÇ verilmez.
comp_ssh_hint() {
    local f="$1" msg first
    [ -s "$f" ] || return 0
    msg=$(head -c 2000 "$f" | LC_ALL=C tr -d '\000-\010\013-\037\177')
    case "$msg" in
        *"REMOTE HOST IDENTIFICATION HAS CHANGED"*|*"Offending "*" key in "*)
            echo "Sunucunun host key'i DEĞİŞMİŞ. Sunucu yeniden kurulduysa eski kaydı silin (ssh-keygen -R); aksi halde araya girilmiş olabilir, bağlanmayın." ;;
        *"No "*" host key is known"*|*"Host key verification failed"*)
            echo "Host key doğrulanamadı. Önce sshpocket'in Bağlan seçeneğiyle bu sunucuya bir kez bağlanıp host key'i kabul edin." ;;
        *"Too many authentication failures"*)
            echo "Çok fazla kimlik doğrulama denemesi (sunucuda MaxAuthTries düşük olabilir). ssh-agent'ta çok anahtar varsa ssh-add -D ile temizleyin." ;;
        *"Permission denied"*)
            echo "Anahtar reddedildi. Doğru anahtar mı (server.env KEY), sunucuda authorized_keys içinde mi, parolalı anahtar ajanda mı (ssh-add -l)?" ;;
        *"timed out"*)
            echo "Zaman aşımı: sunucuya ulaşılamıyor. VPN/ağ açık mı? Sunucuda güvenlik duvarı (ufw) bu ağdan SSH'a izin vermiyor olabilir (hardening_ssh_allow_from)." ;;
        *"No route to host"*|*"Network is unreachable"*)
            echo "Ağ yolu yok: sunucunun ağına ulaşılamıyor. VPN açık mı, doğru ağda mısınız?" ;;
        *"Connection refused"*)
            echo "Bağlantı reddedildi: sshd çalışmıyor ya da port yanlış (server.env PORT)." ;;
        *"Could not resolve hostname"*)
            echo "Sunucu adı çözülemedi: server.env IP/host değerini kontrol edin." ;;
        *"Connection reset"*|*"closed by remote host"*|*"kex_exchange_identification"*)
            echo "Bağlantı sunucu tarafından kesildi. Çok sayıda başarısız deneme sonrası fail2ban engellemiş olabilir; biraz bekleyip yeniden deneyin." ;;
        *)
            first=$(printf '%s\n' "$msg" | head -n 1 | cut -c1-160)
            [ -z "$first" ] || echo "ssh: $first" ;;
    esac
}

_comp_run_cli_usage() {
    echo "kullanım: ./start.sh --components run <paket> <eylem> <sunucu>" >&2
}

# comp_cli_run <paket> <eylem> <sunucu>
comp_cli_run() {
    if [ $# -ne 3 ]; then _comp_run_cli_usage; return 2; fi
    local id="$1" action="$2" server="$3" plat ver info pkg dir prefix ip user port key values range venv profile ans rc refused="$COMP_EXEC_REFUSED"
    comp_id_valid "$id" || { echo "Geçersiz paket kimliği." >&2; return 2; }
    [[ "$action" =~ ^[a-z][a-z0-9_]{0,63}$ ]] || { echo "Geçersiz eylem adı." >&2; return 2; }
    module_name_valid "$server" || { echo "Geçersiz sunucu adı." >&2; return 2; }

    info=$(comp_prepare "$id") || return "$refused"
    ver=$(comp_info_get "$info" version)
    comp_info_all "$info" action | grep -qxF -- "$action" || { echo "'$id' paketinde '$action' eylemi yok." >&2; return 2; }
    pkg="$(comp_dir)/$id/$ver"

    dir="$(servers_dir)/$server"
    [ -d "$dir" ] || { echo "Sunucu bulunamadı: $server" >&2; return 2; }
    plat=$(comp_vars_platform "$pkg/component.json" "$dir/$id.vars" 2>/dev/null) || plat=""
    if ! comp_action_on_platform "$info" "$action" "$plat"; then
        echo "'$action' eylemi bu sunucunun platformunda ($plat) kullanılamaz; yalnızca: $(comp_action_get "$info" "$action" platforms | sed 's/,/, /g')." >&2
        return 2
    fi
    prefix=$(module_prefix "$server")
    ip=$(srv_get "$dir" "$prefix" IP); user=$(srv_get "$dir" "$prefix" USER); port=$(srv_get "$dir" "$prefix" PORT); port="${port:-22}"
    key=$(srv_get "$dir" "$prefix" KEY)
    if [ -n "$key" ]; then
        if [ "${key:0:1}" = "~" ]; then key="$HOME${key:1}"; fi
    else
        key="$dir/id_ed25519_${server}"
    fi
    ssh_validate_target "$ip" "$user" "$port" 2>/dev/null || { echo "Sunucu bilgisi eksik/geçersiz (IP, kullanıcı, port)." >&2; return "$refused"; }
    [ -f "$key" ] || { echo "SSH key bulunamadı: $key" >&2; return "$refused"; }

    # Python ortamı (yalnızca ansible runner): yoksa AÇIKÇA onay istenir (ağdan indirir). script runner venv gerektirmez.
    venv=""
    if [ "$(comp_info_get "$info" runner)" = "ansible" ]; then
        range=$(comp_info_get "$info" requires.python); range="${range:->=3.12}"
        if ! venv=$(comp_venv_lookup "$id" "$pkg" "$range"); then
            echo "Python ortamı kurulu değil. İlk kullanımda kurulacak: PyPI'dan yalnızca hash'li wheel paketleri (yaklaşık 10 MB indirme," >&2
            echo "diskte yaklaşık 90 MB; ~/.cache/sshpocket altında, sistem Python'una dokunulmaz)." >&2
            printf 'Kurulsun mu? [e/H]: ' >&2
            IFS= read -r ans || { echo >&2; echo "Onay alınamadı." >&2; return "$refused"; }
            case "$ans" in e|E|evet|Evet) ;; *) echo "İptal edildi." >&2; return "$refused" ;; esac
            venv=$(comp_venv_ensure "$id" "$pkg" "$range") || return "$refused"
        fi
    fi

    # İşletim sistemi profili: anahtar ajanda olmalı, host key tanınmalı; sonra salt okunur algılama
    comp_gate_known_host "$ip" "$port" || return "$refused"
    comp_gate_agent "$prefix" "$key" || return "$refused"
    local derr hint
    derr=$(mktemp "${TMPDIR:-/tmp}/sshpocket-sshdetect.XXXXXX") || derr=/dev/null
    if ! profile=$(COMP_DETECT_ERRFILE="$derr" comp_detect_profile "$ip" "$port" "$user" "$key"); then
        echo "Sunucunun işletim sistemi algılanamadı (ssh)." >&2
        hint=$(comp_ssh_hint "$derr")
        [ -z "$hint" ] || echo "  ↳ $hint" >&2
        [ "$derr" = /dev/null ] || rm -f -- "$derr"
        return "$refused"
    fi
    [ "$derr" = /dev/null ] || rm -f -- "$derr"
    echo "Sunucu: $server ($user@$ip:$port)  Profil: $profile  Paket: $id $ver" >&2
    comp_warn_clock_skew "$ip" "$port" "$user" "$key"
    # Ubuntu'da varsayılan sudo-rs, Ansible'ın parolalı sudo'sunda takılır: klasik sudo (sudo.ws) varsa onu kullan
    COMP_CTX_BECOME_EXE=""
    case "$profile" in
        ubuntu-*)
            if comp_detect_sudo_ws "$ip" "$port" "$user" "$key"; then
                COMP_CTX_BECOME_EXE="/usr/bin/sudo.ws"
                echo "  ℹ️  Ubuntu: Ansible sudo-rs yerine klasik sudo (/usr/bin/sudo.ws) ile yetki yükseltir." >&2
            fi ;;
    esac

    values="$dir/$id.vars"
    [ -f "$values" ] || values=""
    COMP_CTX_PKGDIR="$pkg"; COMP_CTX_BINDIR="${venv:+$venv/bin}"; COMP_CTX_SERVER="$server"; COMP_CTX_HOST="$ip"; COMP_CTX_USER="$user"
    COMP_CTX_PORT="$port"; COMP_CTX_KEY="$key"; COMP_CTX_PREFIX="$prefix"; COMP_CTX_PROFILE="$profile"; COMP_CTX_VALUES="$values"
    comp_exec_action "$info" "$id" "$ver" "$action"
    rc=$?
    if [ "$rc" -eq "$refused" ]; then echo "Çalıştırılmadı." >&2; return "$rc"; fi
    # Kayıt (gizli bilgi yok): tarih|paket|sürüm|eylem|sonuç
    ( umask 077; printf '%s|%s|%s|%s|%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$id" "$ver" "$action" "$([ "$rc" -eq 0 ] && echo ok || echo fail)" >> "$dir/components.log" ) || true
    if [ -n "$COMP_EXEC_RESULT" ]; then
        echo "Sonuç:" >&2
        while IFS= read -r ans; do echo "  $ans" >&2; done <<< "$COMP_EXEC_RESULT"
    fi
    if [ -n "${COMP_EXEC_LOGFILE:-}" ]; then
        if [ "$rc" -ne 0 ]; then echo "❌ Eylem başarısız. Tam günlük: $COMP_EXEC_LOGFILE" >&2
        else echo "Günlük: $COMP_EXEC_LOGFILE" >&2; fi
    fi
    return "$rc"
}
