#!/usr/bin/env bash
# ComponentsOnboard.sh - Sunucu eklenirken paketin platform listesi ve zorunlu değişken soruları (docs/sunucu-ekleme-platform-secimi.md §3)
# Paket manifestinde platform_choices yoksa HİÇBİR ŞEY sorulmaz ve yazılmaz (eski paketlerde davranış değişmez).
# Platform DOĞRUDAN sorulur; ssh/sudo gerekmez. Erişimi etkileyen değişkenler (onboarding_vars) asla sessizce yazılmaz:
# list[cidr] için mevcut CIDR editörü (comp_menu_edit_cidr: öneri, /24 uyarısı, sunucudan sorgu) kullanılır.
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsManifest.sh, ComponentsRun.sh, ComponentsVars.sh, ComponentsMenu.sh (comp_menu_edit_cidr), Servers.sh

# _comp_onboard_info <paket> → kurulu sürümün bilgi satırları (kurulu değil/geçersizse boş)
_comp_onboard_info() {
    local ver
    ver=$(comp_installed_version "$1")
    [ -n "$ver" ] || return 0
    comp_manifest_info "$(comp_dir)/$1/$ver/component.json" "$1" "$ver" 2>/dev/null || true
}

# comp_onboard_ids → kurulu ve manifestinde platform_choices olan paket kimlikleri (satır satır)
comp_onboard_ids() {
    local id info
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        info=$(_comp_onboard_info "$id")
        [ -n "$(comp_info_get "$info" platform_choice)" ] && printf '%s\n' "$id"
    done < <(comp_ids 2>/dev/null)
    return 0
}

# _comp_onboard_var_type <manifest> <değişken> → tür (örn. list[cidr])
_comp_onboard_var_type() {
    # awk'ı erken bitirmek (exit) python'a BrokenPipe izi bastırırdı: çıktı tamamen okunur
    comp_vars_describe "$1" | awk -F'\t' -v n="$2" '$1 == n && !d { print $2; d=1 }'
}

# _comp_onboard_ask_platform <bilgi> → seçilen platform kimliği (stdout); "" = şimdi değil (hiçbir şey yazılmaz). Menü stderr'e yazılır.
_comp_onboard_ask_platform() {
    local info="$1" id label desc i ans def=""
    local -a ids=()
    while IFS= read -r id; do [ -n "$id" ] && ids+=("$id"); done < <(comp_info_all "$info" platform_choice)
    {
        echo "Bu sunucu hangi türde?"
        i=1
        for id in "${ids[@]}"; do
            label=$(comp_info_get "$info" "platform_choice.$id.label")
            desc=$(comp_info_get "$info" "platform_choice.$id.description")
            [ "$id" = generic ] && def=$i
            echo "  $i) $label${desc:+ — $desc}"
            i=$((i + 1))
        done
        echo "  b) Bilmiyorum${def:+ (genel / generic seçilir)}"
        echo "  s) Şimdi değil (hiçbir şey yazılmaz; pakete ilk girişte yine sorulur)"
    } >&2
    IFS= read -r -p "Seçim: " ans || return 0
    case "$ans" in
        b|B) [ -n "$def" ] && printf '%s' "generic" ;;
        *[!0-9]*|"") ;;
        *) [ "$ans" -ge 1 ] && [ "$ans" -le "${#ids[@]}" ] && printf '%s' "${ids[$((ans - 1))]}" ;;
    esac
    return 0
}

# _comp_onboard_platform <paket> <bilgi> <manifest> <vars-dosyası> → platformu sorar, özetler, onayla yazar. 0: yazıldı veya zaten vardı · 1: yazılmadı
_comp_onboard_platform() {
    local pkg="$1" info="$2" mf="$3" vf="$4" pv pid k v kv ans out
    local -a todo=()
    pv=$(comp_info_get "$info" platform_var)
    [ -n "$pv" ] || return 0
    if comp_vars_get "$vf" "$pv" >/dev/null 2>&1; then
        echo "$pkg: platform zaten kayıtlı ($pv = $(comp_vars_get "$vf" "$pv")); değiştirmek için paket menüsünden değişkenleri düzenleyin."
        return 0
    fi
    pid=$(_comp_onboard_ask_platform "$info")
    [ -n "$pid" ] || { echo "Platform seçilmedi; hiçbir şey yazılmadı."; return 1; }
    todo+=("$pv=$pid")
    while IFS= read -r kv; do
        [ -n "$kv" ] || continue
        k="${kv%%=*}"
        comp_vars_get "$vf" "$k" >/dev/null 2>&1 && continue   # dosyadaki (elle yazılmış) değer ezilmez
        todo+=("$kv")
    done < <(comp_info_all "$info" "platform_choice.$pid.default")
    echo "Yazılacak satırlar ($(basename "$vf")):"
    for kv in "${todo[@]}"; do echo "  $kv"; done
    IFS= read -r -p "Onaylıyor musunuz? [E/h]: " ans || return 1
    case "$ans" in h|H|hayır|Hayır|n|N) echo "İptal edildi; hiçbir şey yazılmadı."; return 1 ;; esac
    for kv in "${todo[@]}"; do
        k="${kv%%=*}"; v="${kv#*=}"
        if ! out=$(comp_vars_save "$mf" "$vf" "$k" "$v" 2>&1); then echo "⚠️  $k yazılamadı: $out"; return 1; fi
    done
    echo "✅ Kaydedildi."
}

# _comp_onboard_vars <paket> <bilgi> <manifest> <vars-dosyası> <sunucu-klasörü> → onboarding_vars'ı sırayla sorar (dolu olanlar atlanır)
_comp_onboard_vars() {
    local pkg="$1" info="$2" mf="$3" vf="$4" dir="$5" n t v out ans server
    local -a names=()
    server=$(basename "$dir")
    while IFS= read -r n; do [ -n "$n" ] && names+=("$n"); done < <(comp_info_all "$info" onboarding_var)
    for n in ${names[@]+"${names[@]}"}; do
        [ -z "$(comp_vars_get "$vf" "$n" 2>/dev/null)" ] || continue
        t=$(_comp_onboard_var_type "$mf" "$n")
        echo ""
        while true; do
            if [ "$t" = "list[cidr]" ]; then
                # shellcheck disable=SC2034  # comp_menu_edit_cidr (dinamik kapsam) okur
                COMP_MENU_MF="$mf"
                # shellcheck disable=SC2034
                COMP_MENU_VARS="$vf"
                # shellcheck disable=SC2034
                COMP_MENU_DIR="$dir"
                comp_menu_edit_cidr "$n" "$server" || true
            else
                echo "$n — $(comp_vars_describe "$mf" | awk -F'\t' -v n="$n" '$1 == n && !d { print $NF; d=1 }')"
                IFS= read -r -p "Değer (boş = atla): " v || return 1
                if [ -n "$v" ]; then
                    if out=$(comp_vars_check "$mf" "$n" "$v" 2>&1); then comp_vars_set "$vf" "$n" "$v" && echo "✅ Kaydedildi: $n = $v"
                    else echo "⚠️  $out"; fi
                fi
            fi
            [ -z "$(comp_vars_get "$vf" "$n" 2>/dev/null)" ] || break
            echo "⚠️  $n boş. $pkg'ın bu değişkene bağlı eylemleri (önizleme/uygulama) ⛔ kapalı kalır; erişimi kilitleyebilecek bir değer sessizce yazılmaz."
            IFS= read -r -p "Şimdi girmek ister misiniz? [E/h]: " ans || return 1
            case "$ans" in h|H|hayır|Hayır|n|N) break ;; esac
        done
    done
}

# comp_onboard_package <paket> <sunucu-klasörü> → bir paket için platform + onboarding_vars (ModuleGenerator ve ilk-açılış akışı çağırır)
comp_onboard_package() {
    local pkg="$1" dir="$2" info ver mf vf
    info=$(_comp_onboard_info "$pkg")
    [ -n "$(comp_info_get "$info" platform_choice)" ] || return 0
    ver=$(comp_info_get "$info" version)
    mf="$(comp_dir)/$pkg/$ver/component.json"
    vf=$(comp_vars_file "$dir" "$pkg")
    echo ""
    echo "── $pkg: sunucu türü ve zorunlu ayarlar ──"
    _comp_onboard_platform "$pkg" "$info" "$mf" "$vf" || true
    _comp_onboard_vars "$pkg" "$info" "$mf" "$vf" "$dir"
}

# comp_onboard_all <sunucu-klasörü> → platform listesi olan her kurulu paket için bir kez
comp_onboard_all() {
    local pkg
    local -a pkgs=()
    # önce diziye: döngü gövdesi stdin'den okur (süreç ikamesi stdin'i kapatırdı)
    while IFS= read -r pkg; do [ -n "$pkg" ] && pkgs+=("$pkg"); done < <(comp_onboard_ids)
    for pkg in ${pkgs[@]+"${pkgs[@]}"}; do comp_onboard_package "$pkg" "$1"; done
    return 0
}
