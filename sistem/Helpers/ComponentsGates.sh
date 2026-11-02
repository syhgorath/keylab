#!/usr/bin/env bash
# ComponentsGates.sh - Paket eylemi çalıştırmadan ÖNCE geçilecek kapılar (Aşama 3, PR B; ağ yok, VM'e DOKUNMAZ)
# Her kapı 0 (geç) ya da 1 (dur + nedeni stderr'e) döner. Onaylar stdin'den okunur; girdi yoksa (EOF) REDDEDİLİR.
# docs/package-standard.md §5 (confirm, requires), §7 (ön koşullar), §14.
# Gerektirir: Components.sh, ComponentsManifest.sh (comp_info_get/all, comp_action_get); isteğe bağlı: SSHHelper.sh (ssh_agent_add_retry)

COMP_REQUIRES_TTL="${COMP_REQUIRES_TTL:-1800}"   # requires eylemlerinin tazelik süresi (sn): 30 dk

_comp_gate_state_dir() { printf '%s' "${SSHPOCKET_STATE_DIR:-$HOME/.local/share/sshpocket}"; }
_comp_gate_name_ok() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$ ]]; }

# comp_gate_mutating <bilgi> <eylem> → 0 ise eylem değiştiricidir (effects boş değil)
comp_gate_mutating() { [ -n "$(comp_action_get "$1" "$2" effects)" ]; }

# comp_gate_not_root <bilgi> <eylem> <kullanıcı> → değiştirici eylemi root ile reddeder
comp_gate_not_root() {
    if [ "$3" = "root" ] && comp_gate_mutating "$1" "$2"; then
        echo "Değiştirici eylem root kullanıcıyla çalıştırılmaz; yetkili bir kullanıcıyla bağlanın (sudo)." >&2
        return 1
    fi
    return 0
}

# comp_gate_known_host <host> <port> [known_hosts] → sunucunun host key'i daha önce kabul edilmiş mi
# Host key doğrulaması kapatılmaz; önce normal bir ssh bağlantısıyla kabul edilmeli.
comp_gate_known_host() {
    local host="$1" port="$2" kh="${3:-${SSHPOCKET_KNOWN_HOSTS:-$HOME/.ssh/known_hosts}}" q="$1"
    [ "$port" = "22" ] || q="[$host]:$port"
    if [ -f "$kh" ] && ssh-keygen -F "$q" -f "$kh" >/dev/null 2>&1; then return 0; fi
    echo "Sunucunun host key'i tanınmıyor ($q). Önce sshpocket ile bir kez bağlanıp host key'i kabul edin." >&2
    return 1
}

# comp_gate_agent <modül-öneki> <anahtar-yolu> → anahtar ssh-agent'ta (değilse eklemeyi dener)
comp_gate_agent() {
    local prefix="$1" key="$2" fp
    fp=$(ssh-keygen -lf "$key" 2>/dev/null | awk '{print $2}')
    [ -n "$fp" ] || { echo "Anahtar okunamadı: $key" >&2; return 1; }
    if ssh-add -l 2>/dev/null | grep -qF -- "$fp"; then return 0; fi
    if declare -F ssh_agent_add_retry >/dev/null; then ssh_agent_add_retry "$prefix" "$key" >/dev/null || true; fi
    if ssh-add -l 2>/dev/null | grep -qF -- "$fp"; then return 0; fi
    echo "Anahtar ssh-agent'ta değil (ssh-add -l). Erişim doğrulanamayacağı için durduruldu." >&2
    return 1
}

# comp_gate_profile <bilgi> <algılanan-profil> → profil pakette 'supported' mı ('planned' ve bilinmeyen reddedilir)
comp_gate_profile() {
    local info="$1" detected="$2" line st
    [ -n "$detected" ] || { echo "Sunucunun işletim sistemi profili belirlenemedi." >&2; return 1; }
    while IFS= read -r line; do
        [ "${line%%:*}" = "$detected" ] || continue
        st="${line#*:}"
        if [ "$st" = "supported" ]; then return 0; fi
        echo "Profil '$detected' pakette '$st' durumunda; desteklenmiyor." >&2
        return 1
    done < <(comp_info_all "$info" profile)
    echo "Profil '$detected' bu paketin profilleri arasında yok." >&2
    return 1
}

# comp_gate_record <paket> <sunucu> <sürüm> <eylem> <ok|fail> → sonucu kullanıcı klasörüne işler (dizin 0700, dosya 0600)
comp_gate_record() {
    local id="$1" server="$2" ver="$3" action="$4" res="$5" dir
    if ! comp_id_valid "$id" || ! _comp_gate_name_ok "$server" || ! comp_version_valid "$ver" \
        || ! [[ "$action" =~ ^[a-z][a-z0-9_]{0,63}$ ]] || { [ "$res" != ok ] && [ "$res" != fail ]; }; then
        echo "kayıt girdisi geçersiz" >&2; return 1
    fi
    dir=$(comp_gate_state_ensure "$id" "$server") || return 1
    ( umask 077; printf '%s|%s|%s|%s\n' "$(date +%s)" "$ver" "$action" "$res" >> "$dir/results" ) || return 1
}

# comp_gate_state_ensure <paket> <sunucu> → kullanıcı durum klasörünü (0700) oluşturur ve yolunu yazar
comp_gate_state_ensure() {
    local id="$1" server="$2" root dir
    if ! comp_id_valid "$id" || ! _comp_gate_name_ok "$server"; then echo "durum klasörü girdisi geçersiz" >&2; return 1; fi
    root="$(_comp_gate_state_dir)"
    dir="$root/$id/$server"
    ( umask 077; mkdir -p "$dir" && chmod 700 "$root" "$root/$id" "$dir" ) || return 1
    printf '%s' "$dir"
}

# comp_gate_requires <bilgi> <paket> <sunucu> <sürüm> <eylem> → requires eylemleri taze (<= TTL) ve başarılı mı
comp_gate_requires() {
    local info="$1" id="$2" server="$3" ver="$4" action="$5" req f now ts v a r found
    local -a reqs
    _comp_gate_name_ok "$server" || { echo "geçersiz sunucu adı" >&2; return 1; }
    f="$(_comp_gate_state_dir)/$id/$server/results"
    now=$(date +%s)
    IFS=',' read -r -a reqs <<< "$(comp_action_get "$info" "$action" requires)"
    for req in ${reqs[@]+"${reqs[@]}"}; do
        found=0
        if [ -f "$f" ]; then
            while IFS='|' read -r ts v a r; do
                [[ "$ts" =~ ^[0-9]+$ ]] || continue
                [ "$v" = "$ver" ] && [ "$a" = "$req" ] && [ "$r" = "ok" ] || continue
                [ "$ts" -le $((now + 60)) ] && [ $((now - ts)) -le "$COMP_REQUIRES_TTL" ] && found=1
            done < "$f"
        fi
        if [ "$found" -ne 1 ]; then
            echo "'$action' için önce '$req' başarıyla çalıştırılmalı (bu sunucu ve sürüm, son $((COMP_REQUIRES_TTL / 60)) dakika içinde)." >&2
            return 1
        fi
    done
    return 0
}

# comp_gate_requires_age <paket> <sunucu> <sürüm> <eylem> → o eylemin en son BAŞARILI (ok) kaydının yaşı (sn); kayıt yoksa 1
# (gelecekteki zaman damgaları sayılmaz). Tazelik kararı çağırana aittir (COMP_REQUIRES_TTL ile karşılaştırın).
comp_gate_requires_age() {
    local id="$1" server="$2" ver="$3" action="$4" f now ts v a r best=""
    _comp_gate_name_ok "$server" || return 1
    f="$(_comp_gate_state_dir)/$id/$server/results"
    [ -f "$f" ] || return 1
    now=$(date +%s)
    while IFS='|' read -r ts v a r; do
        [[ "$ts" =~ ^[0-9]+$ ]] || continue
        [ "$v" = "$ver" ] && [ "$a" = "$action" ] && [ "$r" = "ok" ] || continue
        [ "$ts" -le $((now + 60)) ] || continue
        if [ -z "$best" ] || [ "$ts" -gt "$best" ]; then best="$ts"; fi
    done < "$f"
    [ -n "$best" ] || return 1
    printf '%s' $((now - best))
}

# _comp_gate_ask <istem> → stdin'den bir satır okur; girdi yoksa 1
_comp_gate_ask() {
    local ans
    printf '%s' "$1" >&2
    IFS= read -r ans || { echo >&2; return 1; }
    printf '%s' "$ans"
}

# comp_gate_confirm <bilgi> <eylem> <sunucu> → effects + uyarıları gösterir, confirm kuralını uygular
# none: onaysız · yes-no: e/evet · type-server-name: sunucu adı birebir yazılır (3 deneme). EOF = ret.
comp_gate_confirm() {
    local info="$1" action="$2" server="$3" mode eff w ans try
    mode=$(comp_action_get "$info" "$action" confirm)
    eff=$(comp_action_get "$info" "$action" effects)
    {
        echo "Eylem: $action   Sunucu: $server"
        echo "Etkiler: ${eff:-yok (salt okunur)}"
        while IFS= read -r w; do [ -n "$w" ] && echo "  ⚠️  $w"; done < <(comp_info_all "$info" "action.$action.warning")
    } >&2
    case "$mode" in
        none) return 0 ;;
        yes-no)
            ans=$(_comp_gate_ask "Devam edilsin mi? [e/H]: ") || { echo "Onay alınamadı." >&2; return 1; }
            case "$ans" in e|E|evet|Evet) return 0 ;; esac
            echo "İptal edildi." >&2; return 1 ;;
        type-server-name)
            # yazım hatasına 3 deneme; girdi bitince (EOF) hemen vazgeçilir. Ad yine birebir yazılmak zorundadır.
            for try in 1 2 3; do
                ans=$(_comp_gate_ask "Onaylamak için sunucu adını yazın ($server): ") || { echo "Onay alınamadı." >&2; return 1; }
                [ "$ans" = "$server" ] && return 0
                [ "$try" -lt 3 ] && echo "Sunucu adı uyuşmadı ($try/3); tekrar deneyin." >&2
            done
            echo "Sunucu adı 3 kez uyuşmadı; iptal edildi." >&2; return 1 ;;
        *) echo "Bilinmeyen onay kuralı: $mode" >&2; return 1 ;;
    esac
}

# comp_gate_required_applies <bilgi> <eylem> → 0 ise zorunlu değişken eksikliği bu eylemi engeller: eylem değiştiriciyse YA DA başka bir
# eylemin ön koşuluysa (preview). Salt okunur durum/rapor eylemleri (rollback_status vb.) zorunlu değişken olmadan da çalışır.
comp_gate_required_applies() {
    local info="$1" action="$2" a r
    comp_gate_mutating "$info" "$action" && return 0
    while IFS= read -r a; do
        [ -n "$a" ] || continue
        for r in $(comp_action_get "$info" "$a" requires | tr ',' ' '); do
            [ "$r" = "$action" ] && return 0
        done
    done <<< "$(comp_info_all "$info" action)"
    return 1
}

# comp_gate_unmet_vars <bilgi> <eylem> <değerler-dosyası> → manifestteki action.<eylem>.requires_vars içinden dosyada BOŞ ya da YOK olanları (satır satır) yazar.
# required'dan farkı: yalnızca bu eylemi kapatır (audit/rollback gibi eylemler etkilenmez). Alan yoksa hiçbir şey yazmaz (eski paketler değişmez).
comp_gate_unmet_vars() {
    local info="$1" action="$2" vf="$3" n v
    for n in $(comp_action_get "$info" "$action" requires_vars | tr ',' ' '); do
        v=$(comp_vars_get "$vf" "$n" 2>/dev/null) || v=""
        [ -n "$v" ] || printf '%s\n' "$n"
    done
}

# comp_gate_requires_vars <bilgi> <eylem> <değerler-dosyası> → 0: tamam · 1: eksik değişken var (neden ve ne yapılacağı stderr'e)
comp_gate_requires_vars() {
    local miss
    miss=$(comp_gate_unmet_vars "$1" "$2" "$3" | tr '\n' ' ')
    [ -z "${miss// }" ] && return 0
    {
        echo "⛔ '$2' çalıştırılmadı: değeri boş değişken: ${miss% }. Bu eylem için girilmesi zorunlu (erişim kaybını önlemek için)."
        echo "   Menüde 'Değişkenler' ekranından girin ya da $3 dosyasına '${miss%% *}=<değer>' satırı ekleyin."
    } >&2
    return 1
}

# comp_gate_warn_applies <bilgi> <eylem> → 0 ise warn_if ("riskli değer") onayı bu eylemde sorulur: YALNIZCA yapılandırmayı değiştiren
# eylemlerde. Salt okunur eylemler (preview, durum) ve yalnızca araç kuran eylemler (effects yalnız installs-package, örn. audit) sormaz;
# risk onayı asıl değişikliğin yapıldığı eylemde (apply) alınır.
comp_gate_warn_applies() {
    local eff e
    eff=$(comp_action_get "$1" "$2" effects)
    [ -n "$eff" ] || return 1
    for e in $(printf '%s' "$eff" | tr ',' ' '); do
        [ "$e" = "installs-package" ] || return 0
    done
    return 1
}

# comp_gate_warn_vars <ad>... → warn_if eşleşen her değişken için ayrıca onay ister (e/evet)
comp_gate_warn_vars() {
    local n ans
    for n in "$@"; do
        ans=$(_comp_gate_ask "⚠️  '$n' riskli bir değerde (paket uyarısı). Yine de devam edilsin mi? [e/H]: ") || { echo "Onay alınamadı." >&2; return 1; }
        case "$ans" in e|E|evet|Evet) ;; *) echo "İptal edildi." >&2; return 1 ;; esac
    done
    return 0
}
